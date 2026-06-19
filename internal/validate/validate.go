package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tomnagengast/compactor/internal/reference"
)

type Manifest struct {
	Agent              string     `json:"agent"`
	SessionID          string     `json:"session_id"`
	CWD                string     `json:"cwd"`
	SessionDir         string     `json:"session_dir"`
	PendingContextPath string     `json:"pending_context_path"`
	Documents          []Document `json:"documents"`
}

type Document struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type Report struct {
	ManifestPath string
	Checks       []Check
}

type Check struct {
	Name    string
	OK      bool
	Message string
}

func Run(target string) (Report, error) {
	manifestPath := target
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		manifestPath = filepath.Join(target, "manifest.json")
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return Report{}, fmt.Errorf("read manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Report{}, fmt.Errorf("decode manifest: %w", err)
	}
	if manifest.SessionDir == "" {
		manifest.SessionDir = filepath.Dir(manifestPath)
	}
	if manifest.CWD == "" {
		manifest.CWD = cwdFromSessionDir(manifest.SessionDir)
	}

	report := Report{ManifestPath: manifestPath}
	report.check("manifest", true, manifestPath)
	validateRequiredFields(&report, manifest)
	validateDocuments(&report, manifest)
	validatePendingContext(&report, manifest)
	validateRefs(&report, manifest)
	return report, nil
}

func (report Report) OK() bool {
	for _, check := range report.Checks {
		if !check.OK {
			return false
		}
	}
	return true
}

func (report Report) String() string {
	var b strings.Builder
	b.WriteString("manifest: ")
	b.WriteString(report.ManifestPath)
	b.WriteString("\n")
	for _, check := range report.Checks {
		status := "ok"
		if !check.OK {
			status = "fail"
		}
		b.WriteString("- ")
		b.WriteString(status)
		b.WriteString(" ")
		b.WriteString(check.Name)
		if check.Message != "" {
			b.WriteString(": ")
			b.WriteString(check.Message)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (report *Report) check(name string, ok bool, message string) {
	report.Checks = append(report.Checks, Check{Name: name, OK: ok, Message: message})
}

func validateRequiredFields(report *Report, manifest Manifest) {
	report.check("agent", manifest.Agent != "", manifest.Agent)
	report.check("session-id", manifest.SessionID != "", manifest.SessionID)
	report.check("cwd", manifest.CWD != "", manifest.CWD)
	report.check("session-dir", manifest.SessionDir != "", manifest.SessionDir)
}

func validateDocuments(report *Report, manifest Manifest) {
	seen := map[string]bool{}
	for _, doc := range manifest.Documents {
		if doc.ID == "" {
			report.check("document-id", false, "empty document id")
			continue
		}
		if seen[doc.ID] {
			report.check("document-id", false, "duplicate "+doc.ID)
		}
		seen[doc.ID] = true
		path := docPath(manifest, doc.Path)
		_, err := os.Stat(path)
		report.check("document "+doc.ID, err == nil, path)
	}
}

func validatePendingContext(report *Report, manifest Manifest) {
	if manifest.PendingContextPath == "" {
		report.check("pending-context", false, "missing pending_context_path")
		return
	}
	path := docPath(manifest, manifest.PendingContextPath)
	_, err := os.Stat(path)
	report.check("pending-context", err == nil, path)
}

func validateRefs(report *Report, manifest Manifest) {
	if manifest.Agent == "" || manifest.SessionID == "" || manifest.CWD == "" {
		return
	}
	for _, doc := range manifest.Documents {
		if doc.ID == "" {
			continue
		}
		ref := reference.Session(manifest.Agent, manifest.SessionID, doc.ID)
		_, err := reference.Resolve(ref, manifest.CWD, 1)
		report.check("ref "+doc.ID, err == nil, ref)
	}
}

func docPath(manifest Manifest, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(manifest.SessionDir, path)
}

func cwdFromSessionDir(sessionDir string) string {
	marker := filepath.Join(".compactor", "sessions")
	if index := strings.Index(sessionDir, marker); index > 0 {
		return strings.TrimRight(sessionDir[:index], string(filepath.Separator))
	}
	return filepath.Dir(sessionDir)
}

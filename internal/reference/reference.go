package reference

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tomnagengast/compactor/internal/capsule"
)

const DefaultMaxBytes = 12000

type Document struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type manifest struct {
	Documents []Document `json:"documents"`
}

func Session(agent string, sessionID string, docID string) string {
	return "compactor://session/" + escape(agent) + "/" + escape(sessionID) + "/" + escape(docID)
}

func Resolve(ref string, cwd string, maxBytes int) (string, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}

	path, err := pathForRef(ref, cwd)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read reference target: %w", err)
	}
	return strings.TrimSpace(capsule.Trim(string(data), maxBytes)) + "\n", nil
}

func pathForRef(ref string, cwd string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("reference is required")
	}
	if strings.HasPrefix(ref, "compactor://") {
		return pathForCompactorRef(ref, cwd)
	}
	if filepath.IsAbs(ref) {
		return ref, nil
	}
	return filepath.Join(cwd, ref), nil
}

func pathForCompactorRef(ref string, cwd string) (string, error) {
	parsed, err := url.Parse(ref)
	if err != nil {
		return "", fmt.Errorf("parse reference: %w", err)
	}
	if parsed.Scheme != "compactor" || parsed.Host != "session" {
		return "", fmt.Errorf("unsupported compactor reference: %s", ref)
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 3 {
		return "", fmt.Errorf("expected compactor://session/<agent>/<session>/<doc>")
	}
	agent, sessionID, docID := parts[0], parts[1], parts[2]
	if !safePart(agent) || !safePart(sessionID) || !safePart(docID) {
		return "", fmt.Errorf("reference contains unsafe path segment")
	}

	sessionDir := filepath.Join(cwd, ".compactor", "sessions", agent, sessionID)
	manifestPath := filepath.Join(sessionDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", fmt.Errorf("read manifest: %w", err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return "", fmt.Errorf("decode manifest: %w", err)
	}
	for _, doc := range m.Documents {
		if doc.ID != docID {
			continue
		}
		path := doc.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(sessionDir, path)
		}
		return path, nil
	}
	return "", fmt.Errorf("document %q not found in manifest", docID)
}

func safePart(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	return !strings.ContainsAny(value, `/\`)
}

func escape(value string) string {
	return url.PathEscape(value)
}

func ParseMaxBytes(value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse max bytes: %w", err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("max bytes must be positive")
	}
	return n, nil
}

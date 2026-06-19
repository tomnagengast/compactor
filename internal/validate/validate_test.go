package validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tomnagengast/compactor/internal/reference"
)

func TestRunPassesForCompleteSession(t *testing.T) {
	dir := t.TempDir()
	sessionDir := filepath.Join(dir, ".compactor", "sessions", "claude", "session-1")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sessionDir, "index.md"), "index")
	writeFile(t, filepath.Join(sessionDir, "pending-context.md"), reference.Session("claude", "session-1", "index"))
	writeManifest(t, filepath.Join(sessionDir, "manifest.json"), Manifest{
		Agent:              "claude",
		SessionID:          "session-1",
		CWD:                dir,
		SessionDir:         sessionDir,
		PendingContextPath: filepath.Join(sessionDir, "pending-context.md"),
		Documents:          []Document{{ID: "index", Path: filepath.Join(sessionDir, "index.md")}},
	})

	report, err := Run(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK() {
		t.Fatalf("report failed:\n%s", report.String())
	}
}

func TestRunFailsForMissingDocument(t *testing.T) {
	dir := t.TempDir()
	sessionDir := filepath.Join(dir, ".compactor", "sessions", "codex", "session-1")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(sessionDir, "pending-context.md"), "pending")
	writeManifest(t, filepath.Join(sessionDir, "manifest.json"), Manifest{
		Agent:              "codex",
		SessionID:          "session-1",
		CWD:                dir,
		SessionDir:         sessionDir,
		PendingContextPath: filepath.Join(sessionDir, "pending-context.md"),
		Documents:          []Document{{ID: "index", Path: filepath.Join(sessionDir, "index.md")}},
	})

	report, err := Run(filepath.Join(sessionDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if report.OK() {
		t.Fatalf("report unexpectedly passed:\n%s", report.String())
	}
}

func writeManifest(t *testing.T, path string, manifest Manifest) {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(data))
}

func writeFile(t *testing.T, path string, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

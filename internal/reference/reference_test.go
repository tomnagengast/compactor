package reference

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveCompactorReference(t *testing.T) {
	dir := t.TempDir()
	sessionDir := filepath.Join(dir, ".compactor", "sessions", "codex", "session-1")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	docPath := filepath.Join(sessionDir, "timeline.md")
	if err := os.WriteFile(docPath, []byte("# Timeline\n\nDecision: keep refs small.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifestBody, err := json.Marshal(map[string]any{
		"documents": []map[string]string{
			{"id": "timeline", "path": docPath},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessionDir, "manifest.json"), manifestBody, 0o600); err != nil {
		t.Fatal(err)
	}

	text, err := Resolve(Session("codex", "session-1", "timeline"), dir, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Decision: keep refs small") {
		t.Fatalf("resolved text missing document content:\n%s", text)
	}
}

func TestResolveRejectsUnsafeCompactorReference(t *testing.T) {
	_, err := Resolve("compactor://session/codex/../timeline", t.TempDir(), 1000)
	if err == nil {
		t.Fatal("expected unsafe reference error")
	}
}

func TestResolveBoundsOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 200)), 0o600); err != nil {
		t.Fatal(err)
	}

	text, err := Resolve("doc.md", dir, 40)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "truncated") {
		t.Fatalf("resolved text missing truncation marker: %q", text)
	}
}

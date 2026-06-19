package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadTranscriptExtractsTimelineAndFindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	content := strings.Join([]string{
		`{"timestamp":"2026-06-19T12:00:00Z","message":{"role":"user","content":"Let's make this hook driven and keep prompt caching stable."}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"Decision: use PreCompact to write docs."},{"type":"tool_use","name":"apply_patch"}]}}`,
		`{"type":"tool_result","content":"tests passed"}`,
		`not-json`,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	snapshot, err := Read(path, Options{MaxEntries: 10, MaxTextBytes: 120})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.LineCount != 4 {
		t.Fatalf("line count = %d", snapshot.LineCount)
	}
	if len(snapshot.Entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(snapshot.Entries))
	}
	if len(snapshot.Decisions) < 2 {
		t.Fatalf("decisions = %#v", snapshot.Decisions)
	}
	if len(snapshot.ToolResults) < 2 {
		t.Fatalf("tool results = %#v", snapshot.ToolResults)
	}
}

func TestReadTranscriptBoundsEntryText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"role":"user","content":"`+strings.Repeat("x", 100)+`"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	snapshot, err := Read(path, Options{MaxEntries: 10, MaxTextBytes: 40})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Entries[0].Text) > 60 {
		t.Fatalf("entry was not bounded: %q", snapshot.Entries[0].Text)
	}
	if !strings.Contains(snapshot.Entries[0].Text, "truncated") {
		t.Fatalf("bounded entry missing truncation marker: %q", snapshot.Entries[0].Text)
	}
}

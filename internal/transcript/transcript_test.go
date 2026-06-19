package transcript

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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

func TestReadClaudeTranscriptPromotesStableMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "claude.jsonl")
	content := strings.Join([]string{
		`{"type":"user","uuid":"u1","parentUuid":"root","timestamp":"2026-06-19T12:00:00Z","message":{"role":"user","content":"We should keep prompt cache context tiny."}}`,
		`{"type":"assistant","uuid":"a1","parentUuid":"u1","message":{"role":"assistant","content":[{"type":"text","text":"Decision: write bounded docs."},{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"go test ./..."}}]}}`,
		`{"type":"user","uuid":"u2","parentUuid":"a1","toolUseResult":{"stdout":"ok"},"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"tests passed"}]}}`,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	snapshot, err := Read(path, Options{Agent: "claude", MaxEntries: 10, MaxTextBytes: 120})
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshot.Entries[1].ID; got != "a1" {
		t.Fatalf("entry id = %q, want a1", got)
	}
	if got := snapshot.Entries[1].ParentID; got != "u1" {
		t.Fatalf("entry parent = %q, want u1", got)
	}
	if got := snapshot.Entries[1].ToolName; got != "Bash" {
		t.Fatalf("tool name = %q, want Bash", got)
	}
	if !strings.Contains(snapshot.Entries[2].Kind, "tool") {
		t.Fatalf("tool result kind not promoted: %#v", snapshot.Entries[2])
	}
	if len(snapshot.ToolResults) < 2 {
		t.Fatalf("tool results = %#v", snapshot.ToolResults)
	}
}

func TestReadCodexTranscriptPromotesItemsAndBoundaries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "codex.jsonl")
	content := strings.Join([]string{
		`{"timestamp":"2026-06-19T12:00:00Z","type":"response_item","item":{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"Decision: use adapter-specific transcript normalization."}]}}`,
		`{"timestamp":"2026-06-19T12:01:00Z","type":"response_item","item":{"id":"call_1","type":"function_call","name":"shell","arguments":"{}"}}`,
		`{"timestamp":"2026-06-19T12:02:00Z","type":"compaction","payload":{"id":"compact_1","type":"compact_boundary","summary":"Context compaction started."}}`,
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	snapshot, err := Read(path, Options{Agent: "codex", MaxEntries: 10, MaxTextBytes: 120})
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshot.Entries[0].ID; got != "msg_1" {
		t.Fatalf("entry id = %q, want msg_1", got)
	}
	if got := snapshot.Entries[1].ToolName; got != "shell" {
		t.Fatalf("tool name = %q, want shell", got)
	}
	if len(snapshot.Boundaries) != 1 {
		t.Fatalf("boundaries = %#v, want one compact boundary", snapshot.Boundaries)
	}
	if len(snapshot.Decisions) == 0 {
		t.Fatalf("decisions = %#v, want decision candidate", snapshot.Decisions)
	}
}

func TestReadSanitizedClaudeFixture(t *testing.T) {
	snapshot, err := Read(filepath.Join("testdata", "claude-real-sanitized.jsonl"), Options{Agent: "claude", MaxEntries: 20, MaxTextBytes: 160})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.LineCount != 5 {
		t.Fatalf("line count = %d, want 5", snapshot.LineCount)
	}
	if len(snapshot.Entries) != 5 {
		t.Fatalf("entries = %d, want 5", len(snapshot.Entries))
	}
	if got := snapshot.Entries[2].ID; got != "uuid-assistant-001" {
		t.Fatalf("entry id = %q, want uuid-assistant-001", got)
	}
	if got := snapshot.Entries[2].ParentID; got != "uuid-user-001" {
		t.Fatalf("parent id = %q, want uuid-user-001", got)
	}
	if got := snapshot.Entries[2].ToolName; got != "Bash" {
		t.Fatalf("tool name = %q, want Bash", got)
	}
	if len(snapshot.Decisions) < 3 {
		t.Fatalf("decisions = %#v, want at least 3 candidates", snapshot.Decisions)
	}
	if len(snapshot.ToolResults) < 2 {
		t.Fatalf("tool results = %#v, want tool use and result", snapshot.ToolResults)
	}
}

func TestReadSanitizedCodexFixture(t *testing.T) {
	snapshot, err := Read(filepath.Join("testdata", "codex-real-sanitized.jsonl"), Options{Agent: "codex", MaxEntries: 20, MaxTextBytes: 160})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.LineCount != 6 {
		t.Fatalf("line count = %d, want 6", snapshot.LineCount)
	}
	if got := snapshot.Entries[1].ID; got != "msg_001" {
		t.Fatalf("entry id = %q, want msg_001", got)
	}
	if got := snapshot.Entries[3].ToolName; got != "shell" {
		t.Fatalf("tool name = %q, want shell", got)
	}
	if got := snapshot.Entries[4].ParentID; got != "call-alpha-001" {
		t.Fatalf("parent id = %q, want call-alpha-001", got)
	}
	if len(snapshot.Boundaries) != 1 {
		t.Fatalf("boundaries = %#v, want one compact boundary", snapshot.Boundaries)
	}
	if len(snapshot.Decisions) == 0 {
		t.Fatalf("decisions = %#v, want decision candidates", snapshot.Decisions)
	}
}

func TestSanitizedFixturesDoNotContainObviousLeaks(t *testing.T) {
	leakPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`),
		regexp.MustCompile(`/Users/tom\b`),
		regexp.MustCompile(`(?i)\b(nagengast|bajka|skyview)\b`),
		regexp.MustCompile(`(?i)private key`),
		regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
		regexp.MustCompile(`\bsk-[A-Za-z0-9]{20,}\b`),
		regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{20,}\b`),
	}

	err := filepath.WalkDir("testdata", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		for _, pattern := range leakPatterns {
			if match := pattern.FindString(text); match != "" {
				t.Fatalf("%s contains possible private data matching %q", path, pattern.String())
			}
		}
		for lineIndex, line := range strings.Split(text, "\n") {
			if len(line) > 4096 && !strings.Contains(line, "REDACTED") {
				t.Fatalf("%s line %d is oversized without redaction marker", path, lineIndex+1)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

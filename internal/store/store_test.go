package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tomnagengast/compactor/internal/hookio"
	"github.com/tomnagengast/compactor/internal/validate"
)

func TestPreAndPostCompactWriteSessionDocs(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "session.jsonl")
	transcriptBody := strings.Join([]string{
		`{"message":{"role":"user","content":"Let's make this hook driven."}}`,
		`{"message":{"role":"assistant","content":[{"type":"text","text":"Decision: write compact docs in PreCompact."},{"type":"tool_use","name":"go"}]}}`,
		`{"type":"tool_result","content":"tests passed"}`,
	}, "\n")
	if err := os.WriteFile(transcript, []byte(transcriptBody), 0o600); err != nil {
		t.Fatal(err)
	}

	manager := NewManager()
	event := hookio.Event{
		Agent:          hookio.AgentClaude,
		SessionID:      "session-1",
		CWD:            dir,
		TranscriptPath: transcript,
		HookEventName:  "PreCompact",
		Trigger:        "manual",
	}
	if _, err := manager.PreCompact(event); err != nil {
		t.Fatalf("PreCompact returned error: %v", err)
	}

	post := event
	post.HookEventName = "PostCompact"
	post.TranscriptPath = ""
	post.CompactSummary = "Native summary from Claude."
	if _, err := manager.PostCompact(post); err != nil {
		t.Fatalf("PostCompact returned error: %v", err)
	}

	sessionDir := filepath.Join(dir, ".compactor", "sessions", "claude", "session-1")
	for _, name := range []string{"manifest.json", "index.md", "timeline.md", "decisions.md", "tool-results.md", "pending-context.md", "summaries/native.md"} {
		if _, err := os.Stat(filepath.Join(sessionDir, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(sessionDir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Privacy.RawTranscriptStored {
		t.Fatal("raw transcript should not be stored by default")
	}
	if !manifest.Privacy.BoundedTranscriptExtract {
		t.Fatal("bounded transcript extract should be recorded")
	}
	if len(manifest.Events) != 2 {
		t.Fatalf("events = %d, want 2", len(manifest.Events))
	}
	if !hasDocument(manifest, "native-summary") {
		t.Fatalf("manifest documents missing native-summary: %#v", manifest.Documents)
	}

	pending, err := os.ReadFile(filepath.Join(sessionDir, "pending-context.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(pending), "Native summary from Claude") {
		t.Fatal("pending context should contain references, not full native summary")
	}
	if !strings.Contains(string(pending), ".compactor/sessions/claude/session-1/index.md") {
		t.Fatalf("pending context missing index path:\n%s", pending)
	}

	decisions, err := os.ReadFile(filepath.Join(sessionDir, "decisions.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(decisions), "Decision: write compact docs") {
		t.Fatalf("decisions missing transcript-derived candidate:\n%s", decisions)
	}
	tools, err := os.ReadFile(filepath.Join(sessionDir, "tool-results.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(tools), "tests passed") {
		t.Fatalf("tool results missing transcript-derived candidate:\n%s", tools)
	}
	timeline, err := os.ReadFile(filepath.Join(sessionDir, "timeline.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(timeline), "tool=`go`") {
		t.Fatalf("timeline missing promoted tool name:\n%s", timeline)
	}
}

func TestPreCompactWithSanitizedFixtureValidatesSession(t *testing.T) {
	dir := t.TempDir()
	transcript, err := filepath.Abs(filepath.Join("..", "transcript", "testdata", "claude-real-sanitized.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	manager := NewManager()
	event := hookio.Event{
		Agent:          hookio.AgentClaude,
		SessionID:      "sanitized-session",
		CWD:            dir,
		TranscriptPath: transcript,
		HookEventName:  "PreCompact",
		Trigger:        "manual",
	}
	if _, err := manager.PreCompact(event); err != nil {
		t.Fatalf("PreCompact returned error: %v", err)
	}

	sessionDir := filepath.Join(dir, ".compactor", "sessions", "claude", "sanitized-session")
	report, err := validate.Run(sessionDir)
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK() {
		t.Fatalf("generated session failed validation:\n%s", report.String())
	}

	timeline, err := os.ReadFile(filepath.Join(sessionDir, "timeline.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(timeline), "id=`uuid-assistant-001`") || !strings.Contains(string(timeline), "tool=`Bash`") {
		t.Fatalf("timeline missing promoted fixture metadata:\n%s", timeline)
	}

	pending, err := os.ReadFile(filepath.Join(sessionDir, "pending-context.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(pending), "Decision: write compact docs") {
		t.Fatalf("pending context should contain references, not fixture decisions:\n%s", pending)
	}
}

func hasDocument(manifest Manifest, id string) bool {
	for _, doc := range manifest.Documents {
		if doc.ID == id {
			return true
		}
	}
	return false
}

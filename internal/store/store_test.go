package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tomnagengast/compactor/internal/hookio"
)

func TestPreAndPostCompactWriteSessionDocs(t *testing.T) {
	dir := t.TempDir()
	transcript := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(transcript, []byte(`{"role":"user","content":"hello"}`+"\n"), 0o600); err != nil {
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
	post.CompactSummary = "Native summary from Claude."
	if _, err := manager.PostCompact(post); err != nil {
		t.Fatalf("PostCompact returned error: %v", err)
	}

	sessionDir := filepath.Join(dir, ".compactor", "sessions", "claude", "session-1")
	for _, name := range []string{"manifest.json", "index.md", "timeline.md", "decisions.md", "pending-context.md", "summaries/native.md"} {
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
}

func hasDocument(manifest Manifest, id string) bool {
	for _, doc := range manifest.Documents {
		if doc.ID == id {
			return true
		}
	}
	return false
}

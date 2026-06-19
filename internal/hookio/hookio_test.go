package hookio

import (
	"strings"
	"testing"
)

func TestDecodeEventNormalizesFields(t *testing.T) {
	input := `{
		"session_id": "ABC/123",
		"transcript_path": "session.jsonl",
		"cwd": "/tmp/project",
		"hook_event_name": "PreCompact",
		"trigger": "manual",
		"turn_id": "turn-1",
		"model": "test-model"
	}`

	event, err := DecodeEvent(strings.NewReader(input), AgentClaude)
	if err != nil {
		t.Fatalf("DecodeEvent returned error: %v", err)
	}

	if event.Agent != AgentClaude {
		t.Fatalf("agent = %q, want %q", event.Agent, AgentClaude)
	}
	if event.SessionID != "ABC/123" {
		t.Fatalf("session id = %q", event.SessionID)
	}
	if event.TranscriptPath != "/tmp/project/session.jsonl" {
		t.Fatalf("transcript path = %q", event.TranscriptPath)
	}
	if event.HookEventName != "PreCompact" {
		t.Fatalf("hook event name = %q", event.HookEventName)
	}
}

func TestSanitizePathComponent(t *testing.T) {
	got := SanitizePathComponent("ABC/123 compact")
	if got != "abc-123-compact" {
		t.Fatalf("SanitizePathComponent = %q", got)
	}
}

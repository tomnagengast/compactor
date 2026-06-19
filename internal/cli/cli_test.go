package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookFlowInjectsAdditionalContext(t *testing.T) {
	dir := t.TempDir()
	preInput := strings.NewReader(`{
		"session_id": "session-2",
		"cwd": "` + filepath.ToSlash(dir) + `",
		"hook_event_name": "PreCompact",
		"trigger": "auto"
	}`)
	var preOut bytes.Buffer
	if err := Run([]string{"hook", "codex", "precompact"}, preInput, &preOut, &bytes.Buffer{}, "test"); err != nil {
		t.Fatalf("precompact failed: %v", err)
	}

	injectInput := strings.NewReader(`{
		"session_id": "session-2",
		"cwd": "` + filepath.ToSlash(dir) + `",
		"hook_event_name": "UserPromptSubmit",
		"prompt": "continue"
	}`)
	var injectOut bytes.Buffer
	if err := Run([]string{"hook", "codex", "inject"}, injectInput, &injectOut, &bytes.Buffer{}, "test"); err != nil {
		t.Fatalf("inject failed: %v", err)
	}

	var output map[string]any
	if err := json.Unmarshal(injectOut.Bytes(), &output); err != nil {
		t.Fatalf("decode inject output: %v", err)
	}
	specific := output["hookSpecificOutput"].(map[string]any)
	additional := specific["additionalContext"].(string)
	if !strings.Contains(additional, ".compactor/sessions/codex/session-2/index.md") {
		t.Fatalf("additional context missing index path:\n%s", additional)
	}
	if len(additional) > 2048 {
		t.Fatalf("additional context too large: %d", len(additional))
	}
}

func TestHookDecodeFailureContinues(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"hook", "claude", "precompact"}, strings.NewReader(`not-json`), &out, &bytes.Buffer{}, "test")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	var output map[string]any
	if err := json.Unmarshal(out.Bytes(), &output); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if output["continue"] != true {
		t.Fatalf("continue = %#v, want true", output["continue"])
	}
	if !strings.Contains(output["systemMessage"].(string), "could not decode") {
		t.Fatalf("unexpected system message: %#v", output["systemMessage"])
	}
}

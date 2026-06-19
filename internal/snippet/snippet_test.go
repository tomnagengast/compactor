package snippet

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tomnagengast/compactor/internal/hookio"
)

func TestHooksClaudeSnippet(t *testing.T) {
	text, err := Hooks(hookio.AgentClaude, "/usr/local/bin/compactor")
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		t.Fatalf("snippet is not JSON: %v\n%s", err, text)
	}
	for _, want := range []string{
		`"PreCompact"`,
		`"PostCompact"`,
		`"UserPromptSubmit"`,
		`"SessionStart"`,
		`'/usr/local/bin/compactor' 'hook' 'claude' 'precompact'`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("snippet missing %s:\n%s", want, text)
		}
	}
}

func TestHooksCodexSnippet(t *testing.T) {
	text, err := Hooks(hookio.AgentCodex, "compactor")
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		`'compactor' 'hook' 'codex' 'precompact'`,
		`'compactor' 'hook' 'codex' 'postcompact'`,
		`'compactor' 'hook' 'codex' 'inject'`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("snippet missing %s:\n%s", want, text)
		}
	}
}

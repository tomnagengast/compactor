package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tomnagengast/compactor/internal/hookio"
)

func TestNewPlanProjectTargets(t *testing.T) {
	dir := t.TempDir()

	claude, err := NewPlan(hookio.AgentClaude, ScopeProject, "compactor", dir)
	if err != nil {
		t.Fatal(err)
	}
	if claude.Target != filepath.Join(dir, ".claude", "settings.json") {
		t.Fatalf("claude target = %s", claude.Target)
	}

	codex, err := NewPlan(hookio.AgentCodex, ScopeProject, "compactor", dir)
	if err != nil {
		t.Fatal(err)
	}
	if codex.Target != filepath.Join(dir, ".codex", "hooks.json") {
		t.Fatalf("codex target = %s", codex.Target)
	}
}

func TestPlanWriteAndMergeIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, ".codex", "hooks.json")
	if err := os.MkdirAll(filepath.Dir(existingPath), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo done"}]}]}}`
	if err := os.WriteFile(existingPath, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}

	plan, err := NewPlan(hookio.AgentCodex, ScopeProject, "compactor", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Write(); err != nil {
		t.Fatal(err)
	}
	again, err := NewPlan(hookio.AgentCodex, ScopeProject, "compactor", dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := again.Write(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "echo done") {
		t.Fatalf("existing hook was not preserved:\n%s", text)
	}
	if got := strings.Count(text, "'compactor' 'hook' 'codex' 'precompact'"); got != 1 {
		t.Fatalf("precompact command count = %d\n%s", got, text)
	}
}

func TestDryRunIncludesTargetAndMode(t *testing.T) {
	dir := t.TempDir()
	plan, err := NewPlan(hookio.AgentClaude, ScopeProject, "/tmp/compactor", dir)
	if err != nil {
		t.Fatal(err)
	}
	text, err := plan.DryRun()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "mode: create") || !strings.Contains(text, ".claude/settings.json") {
		t.Fatalf("unexpected dry run:\n%s", text)
	}
}

package cli

import (
	"bytes"
	"encoding/json"
	"os"
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
	if !strings.Contains(additional, "compactor://session/codex/session-2/index") {
		t.Fatalf("additional context missing index ref:\n%s", additional)
	}
	if len(additional) > 2048 {
		t.Fatalf("additional context too large: %d", len(additional))
	}
}

func TestResolveCommandReadsGeneratedReference(t *testing.T) {
	dir := t.TempDir()
	preInput := strings.NewReader(`{
		"session_id": "session-3",
		"cwd": "` + filepath.ToSlash(dir) + `",
		"hook_event_name": "PreCompact",
		"trigger": "manual"
	}`)
	if err := Run([]string{"hook", "claude", "precompact"}, preInput, &bytes.Buffer{}, &bytes.Buffer{}, "test"); err != nil {
		t.Fatalf("precompact failed: %v", err)
	}

	var out bytes.Buffer
	ref := "compactor://session/claude/session-3/index"
	if err := Run([]string{"resolve", ref, "--cwd", dir}, strings.NewReader(""), &out, &bytes.Buffer{}, "test"); err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if !strings.Contains(out.String(), "# Compactor session index") {
		t.Fatalf("resolved output missing index:\n%s", out.String())
	}
}

func TestValidateCommandChecksGeneratedSession(t *testing.T) {
	dir := t.TempDir()
	preInput := strings.NewReader(`{
		"session_id": "session-4",
		"cwd": "` + filepath.ToSlash(dir) + `",
		"hook_event_name": "PreCompact",
		"trigger": "manual"
	}`)
	if err := Run([]string{"hook", "codex", "precompact"}, preInput, &bytes.Buffer{}, &bytes.Buffer{}, "test"); err != nil {
		t.Fatalf("precompact failed: %v", err)
	}

	sessionDir := filepath.Join(dir, ".compactor", "sessions", "codex", "session-4")
	var out bytes.Buffer
	if err := Run([]string{"validate", sessionDir}, strings.NewReader(""), &out, &bytes.Buffer{}, "test"); err != nil {
		t.Fatalf("validate failed: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "- ok ref index") {
		t.Fatalf("validate output missing ref check:\n%s", out.String())
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

func TestHooksSnippetCommand(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"hooks", "snippet", "claude", "--binary", "/tmp/compactor"}, strings.NewReader(""), &out, &bytes.Buffer{}, "test")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(out.String(), `'/tmp/compactor' 'hook' 'claude' 'precompact'`) {
		t.Fatalf("snippet missing command:\n%s", out.String())
	}
}

func TestHooksSnippetPreservesExplicitBinary(t *testing.T) {
	var out bytes.Buffer
	err := Run([]string{"hooks", "snippet", "codex", "--binary", "compactor"}, strings.NewReader(""), &out, &bytes.Buffer{}, "test")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(out.String(), `'compactor' 'hook' 'codex' 'precompact'`) {
		t.Fatalf("snippet did not preserve explicit binary:\n%s", out.String())
	}
}

func TestHooksInstallDryRun(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var out bytes.Buffer
	err := Run([]string{"hooks", "install", "codex", "--binary", "compactor"}, strings.NewReader(""), &out, &bytes.Buffer{}, "test")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(out.String(), "target: "+filepath.Join(dir, ".codex", "hooks.json")) {
		t.Fatalf("dry-run missing target:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "mode: create") {
		t.Fatalf("dry-run missing mode:\n%s", out.String())
	}
	if _, err := os.Stat(filepath.Join(dir, ".codex", "hooks.json")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote hooks file, err=%v", err)
	}
}

func TestHooksInstallWrite(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var out bytes.Buffer
	err := Run([]string{"hooks", "install", "claude", "--binary", "compactor", "--write"}, strings.NewReader(""), &out, &bytes.Buffer{}, "test")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	path := filepath.Join(dir, ".claude", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected hooks file: %v", err)
	}
	if !strings.Contains(string(data), "'compactor' 'hook' 'claude' 'precompact'") {
		t.Fatalf("hooks file missing command:\n%s", data)
	}
}

func TestHooksUninstallWrite(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	var out bytes.Buffer
	if err := Run([]string{"hooks", "install", "codex", "--binary", "compactor", "--write"}, strings.NewReader(""), &out, &bytes.Buffer{}, "test"); err != nil {
		t.Fatalf("install returned error: %v", err)
	}
	out.Reset()
	if err := Run([]string{"hooks", "uninstall", "codex", "--binary", "compactor", "--write"}, strings.NewReader(""), &out, &bytes.Buffer{}, "test"); err != nil {
		t.Fatalf("uninstall returned error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
	if err != nil {
		t.Fatalf("expected hooks file: %v", err)
	}
	if strings.Contains(string(data), "'compactor' 'hook' 'codex'") {
		t.Fatalf("hooks file still contains compactor command:\n%s", data)
	}
}

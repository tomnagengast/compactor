package hookio

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Agent string

const (
	AgentClaude Agent = "claude"
	AgentCodex  Agent = "codex"
)

type Phase string

const (
	PhasePreCompact  Phase = "precompact"
	PhasePostCompact Phase = "postcompact"
	PhaseInject      Phase = "inject"
)

type Event struct {
	Agent          Agent
	SessionID      string
	TranscriptPath string
	CWD            string
	HookEventName  string
	Trigger        string
	TurnID         string
	Source         string
	Model          string
	CompactSummary string
	Raw            map[string]any
}

func ParseAgent(value string) (Agent, error) {
	switch Agent(strings.ToLower(value)) {
	case AgentClaude:
		return AgentClaude, nil
	case AgentCodex:
		return AgentCodex, nil
	default:
		return "", fmt.Errorf("unsupported agent: %s", value)
	}
}

func ParsePhase(value string) (Phase, error) {
	switch Phase(strings.ToLower(value)) {
	case PhasePreCompact:
		return PhasePreCompact, nil
	case PhasePostCompact:
		return PhasePostCompact, nil
	case PhaseInject:
		return PhaseInject, nil
	default:
		return "", fmt.Errorf("unsupported hook phase: %s", value)
	}
}

func DecodeEvent(r io.Reader, agent Agent) (Event, error) {
	var raw map[string]any
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&raw); err != nil {
		return Event{}, fmt.Errorf("decode hook JSON: %w", err)
	}

	cwd := stringField(raw, "cwd")
	if cwd == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Event{}, fmt.Errorf("resolve cwd: %w", err)
		}
		cwd = wd
	}

	transcriptPath := stringField(raw, "transcript_path")
	if transcriptPath != "" && !filepath.IsAbs(transcriptPath) {
		transcriptPath = filepath.Join(cwd, transcriptPath)
	}

	sessionID := stringField(raw, "session_id")
	if sessionID == "" {
		sessionID = fallbackSessionID(cwd, transcriptPath)
	}

	return Event{
		Agent:          agent,
		SessionID:      sessionID,
		TranscriptPath: transcriptPath,
		CWD:            cwd,
		HookEventName:  stringField(raw, "hook_event_name"),
		Trigger:        stringField(raw, "trigger"),
		TurnID:         stringField(raw, "turn_id"),
		Source:         stringField(raw, "source"),
		Model:          stringField(raw, "model"),
		CompactSummary: stringField(raw, "compact_summary"),
		Raw:            raw,
	}, nil
}

func EncodeContinue(w io.Writer, agent Agent, eventName string, additionalContext string) error {
	_ = agent
	out := map[string]any{
		"continue":       true,
		"suppressOutput": true,
	}

	if additionalContext != "" {
		if eventName == "" {
			eventName = "UserPromptSubmit"
		}
		out["hookSpecificOutput"] = map[string]any{
			"hookEventName":     eventName,
			"additionalContext": additionalContext,
		}
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

func EncodeWarning(w io.Writer, message string) error {
	out := map[string]any{
		"continue":       true,
		"suppressOutput": false,
		"systemMessage":  message,
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

func stringField(raw map[string]any, key string) string {
	value, ok := raw[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func fallbackSessionID(cwd string, transcriptPath string) string {
	seed := cwd
	if transcriptPath != "" {
		seed = transcriptPath
	}
	base := filepath.Base(seed)
	base = SanitizePathComponent(base)
	if base == "" {
		return "unknown-session"
	}
	return "session-" + base
}

func SanitizePathComponent(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ToLower(value)
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

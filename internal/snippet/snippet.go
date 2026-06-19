package snippet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tomnagengast/compactor/internal/hookio"
)

func Hooks(agent hookio.Agent, binary string) (string, error) {
	config, err := Config(agent, binary)
	if err != nil {
		return "", err
	}
	return marshal(config)
}

func Config(agent hookio.Agent, binary string) (map[string]any, error) {
	switch agent {
	case hookio.AgentClaude:
		return claude(binary), nil
	case hookio.AgentCodex:
		return codex(binary), nil
	default:
		return nil, fmt.Errorf("unsupported agent: %s", agent)
	}
}

func claude(binary string) map[string]any {
	return map[string]any{
		"hooks": map[string]any{
			"PreCompact": []any{
				matchedCommand("manual|auto", binary, "hook", "claude", "precompact"),
			},
			"PostCompact": []any{
				matchedCommand("manual|auto", binary, "hook", "claude", "postcompact"),
			},
			"UserPromptSubmit": []any{
				command(binary, "hook", "claude", "inject"),
			},
			"SessionStart": []any{
				matchedCommand("compact", binary, "hook", "claude", "inject"),
			},
		},
	}
}

func codex(binary string) map[string]any {
	return map[string]any{
		"hooks": map[string]any{
			"PreCompact": []any{
				matchedCommand("manual|auto", binary, "hook", "codex", "precompact"),
			},
			"PostCompact": []any{
				matchedCommand("manual|auto", binary, "hook", "codex", "postcompact"),
			},
			"UserPromptSubmit": []any{
				command(binary, "hook", "codex", "inject"),
			},
			"SessionStart": []any{
				matchedCommand("compact", binary, "hook", "codex", "inject"),
			},
		},
	}
}

func matchedCommand(matcher string, binary string, args ...string) map[string]any {
	item := command(binary, args...)
	item["matcher"] = matcher
	return item
}

func command(binary string, args ...string) map[string]any {
	return map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": shellCommand(binary, args...),
			},
		},
	}
}

func shellCommand(binary string, args ...string) string {
	parts := []string{quote(binary)}
	for _, arg := range args {
		parts = append(parts, quote(arg))
	}
	return join(parts, " ")
}

func quote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func join(values []string, sep string) string {
	if len(values) == 0 {
		return ""
	}
	var b bytes.Buffer
	for i, value := range values {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(value)
	}
	return b.String()
}

func marshal(value any) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	return string(data), nil
}

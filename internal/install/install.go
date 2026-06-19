package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tomnagengast/compactor/internal/hookio"
	"github.com/tomnagengast/compactor/internal/snippet"
)

type Scope string

const (
	ScopeProject Scope = "project"
	ScopeUser    Scope = "user"
)

type Plan struct {
	Agent  hookio.Agent
	Scope  Scope
	Target string
	Config map[string]any
	Exists bool
}

func NewPlan(agent hookio.Agent, scope Scope, binary string, cwd string) (Plan, error) {
	if scope != ScopeProject && scope != ScopeUser {
		return Plan{}, fmt.Errorf("unsupported install scope: %s", scope)
	}
	target, err := targetPath(agent, scope, cwd)
	if err != nil {
		return Plan{}, err
	}

	config, err := snippet.Config(agent, binary)
	if err != nil {
		return Plan{}, err
	}

	existing, exists, err := readConfig(target)
	if err != nil {
		return Plan{}, err
	}
	merged := merge(existing, config)

	return Plan{
		Agent:  agent,
		Scope:  scope,
		Target: target,
		Config: merged,
		Exists: exists,
	}, nil
}

func (plan Plan) JSON() (string, error) {
	data, err := json.MarshalIndent(plan.Config, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	return string(data), nil
}

func (plan Plan) DryRun() (string, error) {
	body, err := plan.JSON()
	if err != nil {
		return "", err
	}
	status := "create"
	if plan.Exists {
		status = "update"
	}
	return fmt.Sprintf("target: %s\nmode: %s\n\n%s", plan.Target, status, body), nil
}

func (plan Plan) Write() error {
	if err := os.MkdirAll(filepath.Dir(plan.Target), 0o755); err != nil {
		return err
	}
	body, err := plan.JSON()
	if err != nil {
		return err
	}
	return os.WriteFile(plan.Target, []byte(body), 0o600)
}

func targetPath(agent hookio.Agent, scope Scope, cwd string) (string, error) {
	if scope == ScopeProject {
		switch agent {
		case hookio.AgentClaude:
			return filepath.Join(cwd, ".claude", "settings.json"), nil
		case hookio.AgentCodex:
			return filepath.Join(cwd, ".codex", "hooks.json"), nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch agent {
	case hookio.AgentClaude:
		return filepath.Join(home, ".claude", "settings.json"), nil
	case hookio.AgentCodex:
		return filepath.Join(home, ".codex", "hooks.json"), nil
	default:
		return "", fmt.Errorf("unsupported agent: %s", agent)
	}
}

func readConfig(path string) (map[string]any, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, false, nil
		}
		return nil, false, err
	}
	if len(data) == 0 {
		return map[string]any{}, true, nil
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, true, fmt.Errorf("decode %s: %w", path, err)
	}
	return config, true, nil
}

func merge(existing map[string]any, addition map[string]any) map[string]any {
	out := cloneMap(existing)
	existingHooks, _ := out["hooks"].(map[string]any)
	if existingHooks == nil {
		existingHooks = map[string]any{}
	}
	additionHooks, _ := addition["hooks"].(map[string]any)
	for eventName, rawGroups := range additionHooks {
		current := toSlice(existingHooks[eventName])
		for _, group := range toSlice(rawGroups) {
			if !containsCommand(current, group) {
				current = append(current, group)
			}
		}
		existingHooks[eventName] = current
	}
	out["hooks"] = existingHooks
	return out
}

func containsCommand(groups []any, needle any) bool {
	needleCommands := commandStrings(needle)
	if len(needleCommands) == 0 {
		return false
	}
	haystack := map[string]bool{}
	for _, group := range groups {
		for _, command := range commandStrings(group) {
			haystack[command] = true
		}
	}
	for _, command := range needleCommands {
		if !haystack[command] {
			return false
		}
	}
	return true
}

func commandStrings(group any) []string {
	groupMap, ok := group.(map[string]any)
	if !ok {
		return nil
	}
	var commands []string
	for _, hook := range toSlice(groupMap["hooks"]) {
		hookMap, ok := hook.(map[string]any)
		if !ok {
			continue
		}
		command, ok := hookMap["command"].(string)
		if ok && command != "" {
			commands = append(commands, command)
		}
	}
	return commands
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func toSlice(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	default:
		return nil
	}
}

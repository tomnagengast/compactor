package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tomnagengast/compactor/internal/hookio"
	"github.com/tomnagengast/compactor/internal/snippet"
)

type Scope string

const (
	ScopeProject Scope = "project"
	ScopeUser    Scope = "user"
)

type Plan struct {
	Agent       hookio.Agent
	Scope       Scope
	Target      string
	Config      map[string]any
	Exists      bool
	Operation   string
	Diagnostics []Diagnostic
}

type Diagnostic struct {
	Action  string
	Event   string
	Message string
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
	merged, diagnostics := merge(existing, config)

	return Plan{
		Agent:       agent,
		Scope:       scope,
		Target:      target,
		Config:      merged,
		Exists:      exists,
		Operation:   "install",
		Diagnostics: diagnostics,
	}, nil
}

func NewUninstallPlan(agent hookio.Agent, scope Scope, binary string, cwd string) (Plan, error) {
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

	config, diagnostics := remove(existing, config)
	if !exists {
		diagnostics = append(diagnostics, Diagnostic{Action: "missing", Event: "target", Message: target})
	}
	return Plan{
		Agent:       agent,
		Scope:       scope,
		Target:      target,
		Config:      config,
		Exists:      exists,
		Operation:   "uninstall",
		Diagnostics: diagnostics,
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
	if !plan.Exists && plan.Operation == "uninstall" {
		status = "missing"
	}
	return fmt.Sprintf("target: %s\nmode: %s\n%s\n%s", plan.Target, status, diagnosticsText(plan.Diagnostics), body), nil
}

func (plan Plan) Write() error {
	if !plan.Exists && plan.Operation == "uninstall" {
		return nil
	}
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

func merge(existing map[string]any, addition map[string]any) (map[string]any, []Diagnostic) {
	out := cloneMap(existing)
	var diagnostics []Diagnostic
	existingHooks, _ := out["hooks"].(map[string]any)
	if existingHooks == nil {
		if _, ok := out["hooks"]; ok {
			diagnostics = append(diagnostics, Diagnostic{Action: "warn", Event: "hooks", Message: "expected object, replacing with generated hooks"})
		}
		existingHooks = map[string]any{}
	}
	additionHooks, _ := addition["hooks"].(map[string]any)
	processed := map[string]bool{}
	for _, eventName := range sortedKeys(additionHooks) {
		rawGroups := additionHooks[eventName]
		processed[eventName] = true
		current, ok := existingHooks[eventName].([]any)
		if existingHooks[eventName] != nil && !ok {
			diagnostics = append(diagnostics, Diagnostic{Action: "warn", Event: eventName, Message: "expected array, replacing event hooks"})
			current = nil
		}
		for _, group := range toSlice(rawGroups) {
			command := strings.Join(commandStrings(group), ", ")
			if containsCommand(current, group) {
				diagnostics = append(diagnostics, Diagnostic{Action: "skip", Event: eventName, Message: commandOrFallback(command, "command already present")})
			} else {
				current = append(current, group)
				diagnostics = append(diagnostics, Diagnostic{Action: "add", Event: eventName, Message: command})
			}
		}
		existingHooks[eventName] = current
	}
	for _, eventName := range sortedKeys(existingHooks) {
		if processed[eventName] {
			continue
		}
		diagnostics = append(diagnostics, Diagnostic{Action: "preserve", Event: eventName, Message: fmt.Sprintf("%d existing hook group%s", len(toSlice(existingHooks[eventName])), plural(len(toSlice(existingHooks[eventName]))))})
	}
	out["hooks"] = existingHooks
	return out, diagnostics
}

func remove(existing map[string]any, removal map[string]any) (map[string]any, []Diagnostic) {
	out := cloneMap(existing)
	var diagnostics []Diagnostic
	existingHooks, _ := out["hooks"].(map[string]any)
	if existingHooks == nil {
		if _, ok := out["hooks"]; ok {
			diagnostics = append(diagnostics, Diagnostic{Action: "warn", Event: "hooks", Message: "expected object, cannot inspect hooks"})
		}
		return out, diagnostics
	}
	removalHooks, _ := removal["hooks"].(map[string]any)
	for _, eventName := range sortedKeys(removalHooks) {
		rawGroups := removalHooks[eventName]
		current, ok := existingHooks[eventName].([]any)
		if existingHooks[eventName] != nil && !ok {
			diagnostics = append(diagnostics, Diagnostic{Action: "warn", Event: eventName, Message: "expected array, cannot inspect event hooks"})
			continue
		}
		next := current[:0]
		for _, group := range current {
			if !containsCommand(toSlice(rawGroups), group) {
				next = append(next, group)
			}
		}
		for _, group := range toSlice(rawGroups) {
			command := strings.Join(commandStrings(group), ", ")
			if containsCommand(current, group) {
				diagnostics = append(diagnostics, Diagnostic{Action: "remove", Event: eventName, Message: command})
			} else {
				diagnostics = append(diagnostics, Diagnostic{Action: "missing", Event: eventName, Message: command})
			}
		}
		if len(next) == 0 {
			delete(existingHooks, eventName)
		} else {
			existingHooks[eventName] = next
		}
	}
	if len(existingHooks) == 0 {
		delete(out, "hooks")
	} else {
		out["hooks"] = existingHooks
	}
	return out, diagnostics
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

func diagnosticsText(diagnostics []Diagnostic) string {
	var b strings.Builder
	b.WriteString("diagnostics:\n")
	if len(diagnostics) == 0 {
		b.WriteString("- ok: no hook changes detected\n\n")
		return b.String()
	}
	for _, diagnostic := range diagnostics {
		b.WriteString("- ")
		b.WriteString(diagnostic.Action)
		if diagnostic.Event != "" {
			b.WriteByte(' ')
			b.WriteString(diagnostic.Event)
		}
		if diagnostic.Message != "" {
			b.WriteString(": ")
			b.WriteString(diagnostic.Message)
		}
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	return b.String()
}

func sortedKeys(input map[string]any) []string {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func commandOrFallback(command string, fallback string) string {
	if command != "" {
		return command + " already present"
	}
	return fallback
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

package transcript

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Options struct {
	Agent                string
	MaxEntries           int
	MaxTextBytes         int
	LargeToolOutputBytes int
}

type Snapshot struct {
	SourcePath       string
	SourceBytes      int64
	LineCount        int
	Entries          []Entry
	Decisions        []Finding
	ToolResults      []Finding
	Boundaries       []Finding
	LargeToolOutputs []ToolOutput
	ParseError       string
}

type Entry struct {
	Line      int
	ID        string
	ParentID  string
	Timestamp string
	Role      string
	Kind      string
	ToolName  string
	Text      string
}

type Finding struct {
	Line int
	Kind string
	Text string
}

type ToolOutput struct {
	Line      int
	ID        string
	ParentID  string
	Timestamp string
	ToolName  string
	Kind      string
	Bytes     int
	SHA256    string
	Excerpt   string
	Text      string
}

func Read(path string, opts Options) (Snapshot, error) {
	if opts.MaxEntries <= 0 {
		opts.MaxEntries = 80
	}
	if opts.MaxTextBytes <= 0 {
		opts.MaxTextBytes = 500
	}
	if opts.LargeToolOutputBytes <= 0 {
		opts.LargeToolOutputBytes = 4096
	}

	file, err := os.Open(path)
	if err != nil {
		return Snapshot{}, err
	}
	defer file.Close()

	info, _ := file.Stat()
	snapshot := Snapshot{SourcePath: path}
	if info != nil {
		snapshot.SourceBytes = info.Size()
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		snapshot.LineCount++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		entry := entryFromRaw(snapshot.LineCount, raw, opts)
		if entry.Text == "" && entry.Kind == "" {
			continue
		}
		if output, ok := largeToolOutputFromRaw(snapshot.LineCount, raw, entry, opts.LargeToolOutputBytes); ok {
			snapshot.LargeToolOutputs = append(snapshot.LargeToolOutputs, output)
		}
		if len(snapshot.Entries) < opts.MaxEntries {
			snapshot.Entries = append(snapshot.Entries, entry)
		}
		for _, finding := range decisionFindings(entry) {
			if len(snapshot.Decisions) < 40 {
				snapshot.Decisions = append(snapshot.Decisions, finding)
			}
		}
		for _, finding := range toolFindings(entry) {
			if len(snapshot.ToolResults) < 40 {
				snapshot.ToolResults = append(snapshot.ToolResults, finding)
			}
		}
		for _, finding := range boundaryFindings(entry) {
			if len(snapshot.Boundaries) < 40 {
				snapshot.Boundaries = append(snapshot.Boundaries, finding)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func entryFromRaw(line int, raw map[string]any, opts Options) Entry {
	maxTextBytes := opts.MaxTextBytes
	role := firstString(raw, "role")
	if message, ok := raw["message"].(map[string]any); ok {
		if value := firstString(message, "role", "type"); value != "" {
			role = value
		}
	}
	kind := firstString(raw, "type", "event", "hook_event_name")
	id := firstString(raw, "id", "uuid", "item_id")
	parentID := firstString(raw, "parent_id", "parentUuid", "parent_uuid")
	timestamp := firstString(raw, "timestamp", "created_at", "time")
	text := extractText(raw)
	for _, key := range []string{"item", "payload", "event_msg"} {
		nested, ok := raw[key].(map[string]any)
		if !ok {
			continue
		}
		if value := firstString(nested, "id", "uuid"); id == "" && value != "" {
			id = value
		}
		if value := firstString(nested, "parent_id", "parentUuid", "parent_uuid"); parentID == "" && value != "" {
			parentID = value
		}
		if value := firstString(nested, "role"); role == "" && value != "" {
			role = value
		}
		if value := firstString(nested, "type", "event", "kind"); (kind == "" || kind == "response_item") && value != "" {
			kind = value
		}
		if text == "" {
			text = extractText(nested)
		}
		if text == "" {
			text = firstString(nested, "summary", "text", "msg")
		}
	}
	if role == "" {
		role = kind
	}
	if text == "" {
		text = firstString(raw, "summary", "text", "msg")
	}
	entry := Entry{
		Line:      line,
		ID:        id,
		ParentID:  parentID,
		Timestamp: timestamp,
		Role:      normalize(role),
		Kind:      normalize(kind),
		ToolName:  toolNameFromRaw(raw),
		Text:      trimText(text, maxTextBytes),
	}
	adaptEntry(&entry, raw, opts.Agent, maxTextBytes)
	return entry
}

func adaptEntry(entry *Entry, raw map[string]any, agent string, maxTextBytes int) {
	switch normalize(agent) {
	case "claude":
		adaptClaude(entry, raw, maxTextBytes)
	case "codex":
		adaptCodex(entry, raw, maxTextBytes)
	default:
		if looksClaude(raw) {
			adaptClaude(entry, raw, maxTextBytes)
			return
		}
		if looksCodex(raw) {
			adaptCodex(entry, raw, maxTextBytes)
		}
	}
}

func adaptClaude(entry *Entry, raw map[string]any, maxTextBytes int) {
	if entry.ID == "" {
		entry.ID = firstString(raw, "uuid")
	}
	if entry.ParentID == "" {
		entry.ParentID = firstString(raw, "parentUuid")
	}
	if _, ok := raw["toolUseResult"]; ok {
		if entry.Kind == "" || entry.Kind == entry.Role {
			entry.Kind = "tool-result"
		}
		if entry.Text == "" {
			entry.Text = trimText("[tool result metadata]", maxTextBytes)
		}
	}
}

func adaptCodex(entry *Entry, raw map[string]any, maxTextBytes int) {
	_ = maxTextBytes
	for _, key := range []string{"item", "payload", "event_msg"} {
		nested, ok := raw[key].(map[string]any)
		if !ok {
			continue
		}
		if entry.ToolName == "" {
			entry.ToolName = toolNameFromContent(nested)
		}
		if value := firstString(nested, "call_id"); entry.ParentID == "" && value != "" {
			entry.ParentID = value
		}
	}
	if containsCompact(entry.Kind) || containsCompact(entry.Text) {
		if entry.Kind == "" {
			entry.Kind = "compact-boundary"
		}
	}
}

func looksClaude(raw map[string]any) bool {
	return firstString(raw, "uuid", "parentUuid", "sessionId") != ""
}

func looksCodex(raw map[string]any) bool {
	if _, ok := raw["item"]; ok {
		return true
	}
	if _, ok := raw["payload"]; ok {
		return true
	}
	return firstString(raw, "event_msg") != ""
}

func extractText(raw map[string]any) string {
	if value, ok := raw["content"]; ok {
		return contentText(value)
	}
	if message, ok := raw["message"].(map[string]any); ok {
		if value, ok := message["content"]; ok {
			return contentText(value)
		}
	}
	if value, ok := raw["result"]; ok {
		return contentText(value)
	}
	if value, ok := raw["output"]; ok {
		return contentText(value)
	}
	return ""
}

func contentText(value any) string {
	switch typed := value.(type) {
	case string:
		return compactWhitespace(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			text := contentText(item)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	case map[string]any:
		itemType := firstString(typed, "type")
		switch itemType {
		case "text", "input_text", "output_text":
			return firstString(typed, "text")
		case "tool_use":
			name := firstString(typed, "name")
			if name == "" {
				return "[tool use]"
			}
			return "[tool use: " + name + "]"
		case "tool_result":
			text := contentText(typed["content"])
			if text == "" {
				return "[tool result]"
			}
			return "[tool result] " + text
		case "function_call", "tool_call", "local_shell_call", "mcp_tool_call":
			name := firstString(typed, "name", "tool_name")
			if name == "" {
				return "[" + normalize(itemType) + "]"
			}
			return "[" + normalize(itemType) + ": " + name + "]"
		case "message":
			return contentText(typed["content"])
		default:
			if text := firstString(typed, "text", "content", "summary"); text != "" {
				return text
			}
		}
	}
	return ""
}

func largeToolOutputFromRaw(line int, raw map[string]any, entry Entry, threshold int) (ToolOutput, bool) {
	if threshold <= 0 || !isToolResultEntry(entry, raw) {
		return ToolOutput{}, false
	}
	text := rawToolOutput(raw)
	if len(text) < threshold {
		return ToolOutput{}, false
	}
	sum := sha256.Sum256([]byte(text))
	return ToolOutput{
		Line:      line,
		ID:        entry.ID,
		ParentID:  entry.ParentID,
		Timestamp: entry.Timestamp,
		ToolName:  entry.ToolName,
		Kind:      entry.Kind,
		Bytes:     len(text),
		SHA256:    hex.EncodeToString(sum[:]),
		Excerpt:   trimText(text, 240),
		Text:      text,
	}, true
}

func isToolResultEntry(entry Entry, raw map[string]any) bool {
	lower := strings.ToLower(entry.Role + " " + entry.Kind + " " + entry.Text)
	if strings.Contains(lower, "tool-result") ||
		strings.Contains(lower, "tool_result") ||
		strings.Contains(lower, "[tool result") ||
		entry.Role == "tool" ||
		raw["toolUseResult"] != nil {
		return true
	}
	for _, key := range []string{"item", "payload", "event_msg"} {
		nested, ok := raw[key].(map[string]any)
		if !ok {
			continue
		}
		itemType := normalize(firstString(nested, "type"))
		if strings.Contains(itemType, "output") || strings.Contains(itemType, "result") {
			return true
		}
	}
	return false
}

func rawToolOutput(raw map[string]any) string {
	var parts []string
	if value, ok := raw["toolUseResult"].(map[string]any); ok {
		for _, key := range []string{"stdout", "stderr", "output", "error"} {
			if text := rawString(value[key]); text != "" {
				parts = append(parts, text)
			}
		}
	}
	for _, key := range []string{"content", "result", "output", "stdout", "stderr"} {
		if text := rawContentString(raw[key]); text != "" {
			parts = append(parts, text)
		}
	}
	if message, ok := raw["message"].(map[string]any); ok {
		if text := rawContentString(message["content"]); text != "" {
			parts = append(parts, text)
		}
	}
	for _, key := range []string{"item", "payload", "event_msg"} {
		nested, ok := raw[key].(map[string]any)
		if !ok {
			continue
		}
		for _, nestedKey := range []string{"content", "result", "output", "stdout", "stderr"} {
			if text := rawContentString(nested[nestedKey]); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func rawContentString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []any:
		var parts []string
		for _, item := range typed {
			if text := rawContentString(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	case map[string]any:
		itemType := normalize(firstString(typed, "type"))
		if itemType != "" && !(strings.Contains(itemType, "result") || strings.Contains(itemType, "output") || strings.Contains(itemType, "tool")) {
			return ""
		}
		for _, key := range []string{"content", "text", "output", "stdout", "stderr", "error"} {
			if text := rawContentString(typed[key]); text != "" {
				return text
			}
		}
	}
	return ""
}

func rawString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func decisionFindings(entry Entry) []Finding {
	if entry.Text == "" {
		return nil
	}
	lower := strings.ToLower(entry.Text)
	keywords := []string{
		"decision", "decide", "decided", "we should", "we will", "let's", "must ",
		"constraint", "non-goal", "open question", "next slice", "next step",
	}
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return []Finding{{Line: entry.Line, Kind: "candidate", Text: entry.Text}}
		}
	}
	return nil
}

func toolFindings(entry Entry) []Finding {
	if entry.Text == "" && entry.ToolName == "" {
		return nil
	}
	lower := strings.ToLower(entry.Text)
	if strings.Contains(lower, "[tool use") || strings.Contains(lower, "[tool result") || entry.Role == "tool" || strings.Contains(entry.Kind, "tool") {
		text := entry.Text
		if text == "" {
			text = "[tool: " + entry.ToolName + "]"
		}
		return []Finding{{Line: entry.Line, Kind: "tool", Text: text}}
	}
	if entry.ToolName != "" {
		return []Finding{{Line: entry.Line, Kind: "tool", Text: "[tool: " + entry.ToolName + "] " + entry.Text}}
	}
	return nil
}

func boundaryFindings(entry Entry) []Finding {
	if containsCompact(entry.Kind) || containsCompact(entry.Text) {
		text := entry.Text
		if text == "" {
			text = entry.Kind
		}
		return []Finding{{Line: entry.Line, Kind: "compact-boundary", Text: text}}
	}
	return nil
}

func toolNameFromRaw(raw map[string]any) string {
	for _, key := range []string{"tool_name", "name"} {
		if value := firstString(raw, key); value != "" {
			return value
		}
	}
	for _, key := range []string{"content", "message", "item", "payload", "event_msg"} {
		value, ok := raw[key]
		if !ok {
			continue
		}
		if name := toolNameFromContent(value); name != "" {
			return name
		}
	}
	return ""
}

func toolNameFromContent(value any) string {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if name := toolNameFromContent(item); name != "" {
				return name
			}
		}
	case map[string]any:
		itemType := normalize(firstString(typed, "type"))
		if strings.Contains(itemType, "tool") || strings.Contains(itemType, "function") || strings.Contains(itemType, "shell") {
			if name := firstString(typed, "name", "tool_name"); name != "" {
				return name
			}
		}
		for _, key := range []string{"content", "message", "item", "payload"} {
			if name := toolNameFromContent(typed[key]); name != "" {
				return name
			}
		}
	}
	return ""
}

func containsCompact(value string) bool {
	lower := strings.ToLower(value)
	return lower == "compact" ||
		strings.Contains(lower, "compaction") ||
		strings.Contains(lower, "compact_boundary") ||
		strings.Contains(lower, "compact-boundary") ||
		strings.Contains(lower, "compact boundary") ||
		strings.Contains(lower, "precompact") ||
		strings.Contains(lower, "postcompact")
}

func firstString(raw map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			return compactWhitespace(typed)
		default:
			return compactWhitespace(fmt.Sprint(typed))
		}
	}
	return ""
}

func normalize(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func trimText(value string, maxBytes int) string {
	value = compactWhitespace(value)
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	if maxBytes < 16 {
		return value[:maxBytes]
	}
	return strings.TrimSpace(value[:maxBytes-15]) + " [truncated]"
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

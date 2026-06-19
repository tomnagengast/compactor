package transcript

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Options struct {
	MaxEntries   int
	MaxTextBytes int
}

type Snapshot struct {
	SourcePath  string
	SourceBytes int64
	LineCount   int
	Entries     []Entry
	Decisions   []Finding
	ToolResults []Finding
	ParseError  string
}

type Entry struct {
	Line      int
	Timestamp string
	Role      string
	Kind      string
	Text      string
}

type Finding struct {
	Line int
	Kind string
	Text string
}

func Read(path string, opts Options) (Snapshot, error) {
	if opts.MaxEntries <= 0 {
		opts.MaxEntries = 80
	}
	if opts.MaxTextBytes <= 0 {
		opts.MaxTextBytes = 500
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
		entry := entryFromRaw(snapshot.LineCount, raw, opts.MaxTextBytes)
		if entry.Text == "" && entry.Kind == "" {
			continue
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
	}
	if err := scanner.Err(); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func entryFromRaw(line int, raw map[string]any, maxTextBytes int) Entry {
	role := firstString(raw, "role", "type", "event", "hook_event_name")
	if message, ok := raw["message"].(map[string]any); ok {
		if value := firstString(message, "role", "type"); value != "" {
			role = value
		}
	}
	kind := firstString(raw, "type", "event", "hook_event_name")
	timestamp := firstString(raw, "timestamp", "created_at", "time")
	text := extractText(raw)
	if text == "" {
		text = firstString(raw, "summary", "text")
	}
	return Entry{
		Line:      line,
		Timestamp: timestamp,
		Role:      normalize(role),
		Kind:      normalize(kind),
		Text:      trimText(text, maxTextBytes),
	}
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
		case "text":
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
		default:
			if text := firstString(typed, "text", "content", "summary"); text != "" {
				return text
			}
		}
	}
	return ""
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
	if entry.Text == "" {
		return nil
	}
	lower := strings.ToLower(entry.Text)
	if strings.Contains(lower, "[tool use") || strings.Contains(lower, "[tool result") || entry.Role == "tool" || strings.Contains(entry.Kind, "tool") {
		return []Finding{{Line: entry.Line, Kind: "tool", Text: entry.Text}}
	}
	return nil
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

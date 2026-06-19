package extract

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tomnagengast/compactor/internal/transcript"
)

type Result struct {
	Decisions  []Finding
	Tools      []Finding
	Boundaries []Finding
}

type Finding struct {
	ID         string
	Category   string
	Kind       string
	Confidence string
	Line       int
	EntryID    string
	ParentID   string
	Role       string
	ToolName   string
	ToolCallID string
	Text       string
	Reason     string
}

func Analyze(snapshot transcript.Snapshot) Result {
	var result Result
	seen := map[string]bool{}
	for _, entry := range snapshot.Entries {
		for _, finding := range classifyText(entry) {
			key := finding.Category + "\x00" + normalizeText(finding.Text)
			if seen[key] {
				continue
			}
			seen[key] = true
			finding.ID = findingID(finding, len(result.Decisions)+1)
			result.Decisions = append(result.Decisions, finding)
		}
		if finding, ok := classifyTool(entry); ok {
			finding.ID = findingID(finding, len(result.Tools)+1)
			result.Tools = append(result.Tools, finding)
		}
		if finding, ok := classifyBoundary(entry); ok {
			finding.ID = findingID(finding, len(result.Boundaries)+1)
			result.Boundaries = append(result.Boundaries, finding)
		}
	}
	return result
}

func classifyText(entry transcript.Entry) []Finding {
	if entry.Role == "tool" || strings.Contains(entry.Kind, "tool-result") || strings.Contains(entry.Kind, "tool_result") {
		return nil
	}
	if entry.Text == "" || (isToolEntry(entry) && !hasExplicitTextMarker(entry.Text)) {
		return nil
	}
	clauses := splitClauses(entry.Text)
	findings := make([]Finding, 0, len(clauses))
	for _, clause := range clauses {
		category, confidence, reason, ok := classifyClause(clause)
		if !ok {
			continue
		}
		findings = append(findings, baseFinding(entry, category, "candidate", confidence, clause, reason))
	}
	return findings
}

func hasExplicitTextMarker(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "decision:") ||
		strings.Contains(lower, "decided:") ||
		strings.Contains(lower, "open question") ||
		strings.Contains(lower, "next step") ||
		strings.Contains(lower, "next slice")
}

func classifyClause(text string) (string, string, string, bool) {
	lower := strings.ToLower(text)
	if looksLikeNoise(lower) {
		return "", "", "", false
	}
	switch {
	case strings.HasPrefix(lower, "decision:") ||
		strings.HasPrefix(lower, "decided:") ||
		strings.Contains(lower, "we decided") ||
		strings.Contains(lower, "chosen approach"):
		return "decision", "high", "explicit decision marker", true
	case strings.Contains(lower, " we will ") || strings.HasPrefix(lower, "we will ") ||
		strings.Contains(lower, "use ") && strings.Contains(lower, " for ") ||
		strings.Contains(lower, "keep ") && strings.Contains(lower, " as "):
		return "decision", "medium", "durable choice language", true
	case strings.HasPrefix(lower, "open question:") || strings.Contains(lower, "open question"):
		return "open-question", "high", "explicit open question marker", true
	case strings.HasSuffix(lower, "?") && (strings.Contains(lower, "should ") || strings.Contains(lower, "how should") || strings.Contains(lower, "what ")):
		return "open-question", "medium", "question about implementation choice", true
	case containsAnyWord(lower, []string{"must", "never", "only", "avoid"}) ||
		strings.Contains(lower, "must not") ||
		strings.Contains(lower, "do not") ||
		strings.Contains(lower, "local-first") ||
		strings.Contains(lower, "deterministic") ||
		strings.Contains(lower, "bounded") ||
		strings.Contains(lower, "non-goal"):
		return "constraint", "medium", "constraint keyword", true
	case strings.Contains(lower, "next step") ||
		strings.Contains(lower, "next slice") ||
		strings.Contains(lower, "follow up") ||
		containsAnyWord(lower, []string{"todo"}):
		return "next-action", "medium", "next-action marker", true
	case strings.Contains(lower, "we should") || strings.Contains(lower, "let's"):
		return "next-action", "low", "proposal language", true
	default:
		return "", "", "", false
	}
}

func classifyTool(entry transcript.Entry) (Finding, bool) {
	lowerText := strings.ToLower(entry.Text)
	lowerKind := strings.ToLower(entry.Kind)
	if entry.ToolName == "" &&
		!strings.Contains(lowerText, "[tool use") &&
		!strings.Contains(lowerText, "[tool result") &&
		!strings.Contains(lowerKind, "tool") &&
		entry.Role != "tool" {
		return Finding{}, false
	}
	category := "tool-call"
	reason := "tool call marker"
	if strings.Contains(lowerText, "[tool result") || strings.Contains(lowerKind, "result") {
		category = "tool-result"
		reason = "tool result marker"
	}
	return baseFinding(entry, category, "tool", "high", entry.Text, reason), true
}

func classifyBoundary(entry transcript.Entry) (Finding, bool) {
	lower := strings.ToLower(entry.Kind + " " + entry.Text)
	if lower == "" ||
		!(strings.Contains(lower, "compaction") ||
			strings.Contains(lower, "compact_boundary") ||
			strings.Contains(lower, "compact-boundary") ||
			strings.Contains(lower, "compact boundary") ||
			strings.Contains(lower, "precompact") ||
			strings.Contains(lower, "postcompact")) {
		return Finding{}, false
	}
	return baseFinding(entry, "compaction-boundary", "boundary", "high", entry.Text, "compaction marker"), true
}

func baseFinding(entry transcript.Entry, category string, kind string, confidence string, text string, reason string) Finding {
	return Finding{
		Category:   category,
		Kind:       kind,
		Confidence: confidence,
		Line:       entry.Line,
		EntryID:    entry.ID,
		ParentID:   entry.ParentID,
		Role:       entry.Role,
		ToolName:   entry.ToolName,
		ToolCallID: toolCallID(entry),
		Text:       strings.TrimSpace(text),
		Reason:     reason,
	}
}

func toolCallID(entry transcript.Entry) string {
	if entry.ParentID != "" && (strings.Contains(entry.Kind, "result") || strings.Contains(strings.ToLower(entry.Text), "[tool result")) {
		return entry.ParentID
	}
	return entry.ID
}

func findingID(finding Finding, sequence int) string {
	return fmt.Sprintf("%s-%04d-%02d", finding.Category, finding.Line, sequence)
}

func splitClauses(text string) []string {
	parts := clauseSplitRE.Split(text, -1)
	clauses := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		clauses = append(clauses, part)
	}
	return clauses
}

func isToolEntry(entry transcript.Entry) bool {
	lower := strings.ToLower(entry.Role + " " + entry.Kind + " " + entry.Text)
	return entry.ToolName != "" ||
		entry.Role == "tool" ||
		strings.Contains(lower, "[tool use") ||
		strings.Contains(lower, "[tool result") ||
		strings.Contains(lower, "tool-result")
}

func looksLikeNoise(lower string) bool {
	return strings.Contains(lower, "panic:") ||
		strings.Contains(lower, "stack trace") ||
		strings.Contains(lower, "traceback") ||
		strings.Contains(lower, "must have") ||
		strings.HasPrefix(lower, "ok github.com/")
}

func containsAnyWord(text string, words []string) bool {
	for _, word := range words {
		if wordRE(word).MatchString(text) {
			return true
		}
	}
	return false
}

func normalizeText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	text = strings.Join(strings.Fields(text), " ")
	return text
}

func wordRE(word string) *regexp.Regexp {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`)
}

var clauseSplitRE = regexp.MustCompile(`[.;]\s+|\n+`)

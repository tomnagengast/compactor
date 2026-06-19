package extract

import (
	"reflect"
	"testing"

	"github.com/tomnagengast/compactor/internal/transcript"
)

func TestAnalyzeClassifiesDecisionsAndConstraints(t *testing.T) {
	snapshot := transcript.Snapshot{
		Entries: []transcript.Entry{
			{Line: 1, ID: "u1", Role: "user", Text: "Decision: use manifest backed refs. Open question: should output chunks be stored by default?"},
			{Line: 2, ID: "a1", Role: "assistant", Text: "We will keep raw transcript capture disabled by default. The implementation must stay deterministic."},
			{Line: 3, ID: "a2", Role: "assistant", Text: "Next step: add fixture leak tests."},
		},
	}

	result := Analyze(snapshot)
	got := categories(result.Decisions)
	want := []string{"decision", "open-question", "decision", "constraint", "next-action"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("categories = %#v, want %#v", got, want)
	}
	if result.Decisions[0].ID != "decision-0001-01" {
		t.Fatalf("first id = %q", result.Decisions[0].ID)
	}
}

func TestAnalyzeSuppressesCommonFalsePositives(t *testing.T) {
	snapshot := transcript.Snapshot{
		Entries: []transcript.Entry{
			{Line: 1, Role: "assistant", Text: "The package must have been cached already."},
			{Line: 2, Role: "assistant", Text: "panic: stack trace says this line should not be a decision."},
			{Line: 3, Role: "tool", Kind: "tool_result", Text: "Decision: fake decision from tool output."},
		},
	}

	result := Analyze(snapshot)
	if len(result.Decisions) != 0 {
		t.Fatalf("decisions = %#v, want none", result.Decisions)
	}
}

func TestAnalyzeClassifiesToolsAndBoundaries(t *testing.T) {
	snapshot := transcript.Snapshot{
		Entries: []transcript.Entry{
			{Line: 1, ID: "call-1", Role: "assistant", Kind: "function_call", ToolName: "shell", Text: "[function_call: shell]"},
			{Line: 2, ID: "result-1", ParentID: "call-1", Role: "tool", Kind: "tool_result", Text: "[tool result] tests passed"},
			{Line: 3, ID: "compact-1", Kind: "compact_boundary", Text: "Context compaction started."},
		},
	}

	result := Analyze(snapshot)
	if len(result.Tools) != 2 {
		t.Fatalf("tools = %#v, want 2", result.Tools)
	}
	if result.Tools[0].Category != "tool-call" {
		t.Fatalf("first tool category = %q", result.Tools[0].Category)
	}
	if result.Tools[1].Category != "tool-result" || result.Tools[1].ToolCallID != "call-1" {
		t.Fatalf("tool result = %#v", result.Tools[1])
	}
	if len(result.Boundaries) != 1 {
		t.Fatalf("boundaries = %#v, want one", result.Boundaries)
	}
}

func TestAnalyzeIsDeterministic(t *testing.T) {
	snapshot := transcript.Snapshot{
		Entries: []transcript.Entry{{Line: 1, ID: "u1", Role: "user", Text: "Decision: keep output bounded."}},
	}
	first := Analyze(snapshot)
	second := Analyze(snapshot)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Analyze was not deterministic:\n%#v\n%#v", first, second)
	}
}

func categories(findings []Finding) []string {
	out := make([]string, 0, len(findings))
	for _, finding := range findings {
		out = append(out, finding.Category)
	}
	return out
}

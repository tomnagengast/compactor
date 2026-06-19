package version

import (
	"strings"
	"testing"
)

func TestStringUsesCommandAndVersion(t *testing.T) {
	oldVersion, oldCommit, oldDate := Version, Commit, Date
	t.Cleanup(func() {
		Version, Commit, Date = oldVersion, oldCommit, oldDate
	})

	Version = "v0.1.0"
	Commit = "abc123"
	Date = "2026-06-19T00:00:00Z"

	got := String()
	for _, want := range []string{"compactor", "v0.1.0", "abc123", "2026-06-19T00:00:00Z"} {
		if !strings.Contains(got, want) {
			t.Fatalf("String() = %q, missing %q", got, want)
		}
	}
}

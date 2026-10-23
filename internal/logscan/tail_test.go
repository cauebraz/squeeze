package logscan

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTailLinesKeepsErrorLinesTheLineAfterAndTheTail(t *testing.T) {
	t.Parallel()
	lines := []string{"a", "b", "--- FAIL: TestX (0.01s)", "x_test.go:9: got 1 want 2", "c", "d", "e", "f"}
	got, omitted := TailLines(lines, 2, "--max-lines")
	want := []string{"... 2 lines cut (--max-lines 0)", "--- FAIL: TestX (0.01s)", "x_test.go:9: got 1 want 2", "... 2 lines cut (--max-lines 0)", "e", "f"}
	if diff := cmp.Diff(want, got); diff != "" || omitted != 4 {
		t.Errorf("omitted %d, lines (-want +got):\n%s", omitted, diff)
	}
	if got, omitted := TailLines(lines, 0, "--max-lines"); len(got) != len(lines) || omitted != 0 {
		t.Errorf("max 0 cut %d lines", omitted)
	}
	if got, _ := TailLines([]string{"a", "b", "c"}, 1, "--max-log"); !cmp.Equal(got, []string{"... 2 lines cut (--max-log 0)", "c"}) {
		t.Errorf("flag not named: %v", got)
	}
}

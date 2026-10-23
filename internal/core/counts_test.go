package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestJoinCounts(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		parts []Part
		want  string
	}{
		"plural and fixed words": {[]Part{{2, "failure", "failures"}, {1, "error", "errors"}, {3, "skipped", ""}}, "2 failures, 1 error, 3 skipped"},
		"zero parts left out":    {[]Part{{0, "failure", "failures"}, {1, "flaky", ""}}, "1 flaky"},
		"nothing to count":       {[]Part{{0, "test", "tests"}}, ""},
	}
	for name, tt := range tests {
		if diff := cmp.Diff(tt.want, JoinCounts(tt.parts...)); diff != "" {
			t.Errorf("%s: (-want +got):\n%s", name, diff)
		}
	}
}

func TestUnknownNotice(t *testing.T) {
	t.Parallel()
	got := UnknownNotice(map[string]int{"weird (x)": 2, "": 1, "alpha": 1}, "event", "events", "action")
	want := "... 4 events with an unknown action: empty 1, alpha 1, weird [x] 2 (--raw-dir)"
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
	if got := UnknownNotice(map[string]int{"x": 1}, "test", "tests", "status"); got != "... 1 test with an unknown status: x 1 (--raw-dir)" {
		t.Errorf("one: %q", got)
	}
	if got := UnknownNotice(nil, "test", "tests", "status"); got != "" {
		t.Errorf("empty: %q", got)
	}
}

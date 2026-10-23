package core

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSeverityOrder(t *testing.T) {
	t.Parallel()
	o := SeverityOrder{"critical", "high", "low"}
	ranks := map[string]int{"critical": 0, "high": 1, "low": 2, "bogus": 3, "": 3, "CRITICAL": 3}
	for s, want := range ranks {
		if got := o.Rank(s); got != want {
			t.Errorf("Rank(%q) = %d, want %d", s, got, want)
		}
		if got := o.Known(s); got != (want < len(o)) {
			t.Errorf("Known(%q) = %v", s, got)
		}
	}
	tests := []struct {
		s, level string
		want     bool
	}{
		{"critical", "critical", true},
		{"critical", "low", true},
		{"high", "critical", false},
		{"low", "low", true},
		{"bogus", "low", false},
		{"bogus", "critical", false},
		{"bogus", "bogus", false},
		{"low", "bogus", true},
	}
	for _, tt := range tests {
		if got := o.AtLeast(tt.s, tt.level); got != tt.want {
			t.Errorf("AtLeast(%q, %q) = %v, want %v", tt.s, tt.level, got, tt.want)
		}
	}
	got := []string{"bogus", "low", "critical", "", "high"}
	slices.SortStableFunc(got, func(a, b string) int { return o.Rank(a) - o.Rank(b) })
	if diff := cmp.Diff([]string{"critical", "high", "low", "bogus", ""}, got); diff != "" {
		t.Errorf("sorted by rank (-want +got):\n%s", diff)
	}
}

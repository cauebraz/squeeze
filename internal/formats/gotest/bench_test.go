package gotest

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func nestedFailures(n int) []rawEvent {
	events := make([]rawEvent, 0, 5*n+1)
	for i := range n {
		parent := fmt.Sprintf("TestCase%d", i)
		child := parent + "/sub"
		events = append(events,
			rawEvent{Action: "run", Package: "p", Test: parent},
			rawEvent{Action: "run", Package: "p", Test: child},
			rawEvent{Action: "output", Package: "p", Test: child, Output: "    x_test.go:1: boom\n"},
			rawEvent{Action: "fail", Package: "p", Test: child},
			rawEvent{Action: "fail", Package: "p", Test: parent},
		)
	}
	return append(events, rawEvent{Action: "fail", Package: "p"})
}

func BenchmarkBuildNestedFailures(b *testing.B) {
	for _, n := range []int{5000, 10000, 20000} {
		events := nestedFailures(n)
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			for b.Loop() {
				build(events, defaultOptions(), false)
			}
		})
	}
}

func TestNestedFailuresListOnlyTheChild(t *testing.T) {
	t.Parallel()
	r := build(nestedFailures(3), defaultOptions(), false)
	var got []string
	for _, f := range r.Failures {
		got = append(got, f.Test)
	}
	want := []string{"TestCase0/sub", "TestCase1/sub", "TestCase2/sub"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failures (-want +got):\n%s", diff)
	}
	if r.Tests.Failed != 3 {
		t.Errorf("failed = %d, want 3", r.Tests.Failed)
	}
}

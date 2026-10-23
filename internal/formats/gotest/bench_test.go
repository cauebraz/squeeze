package gotest

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func nestedFailures(n int) []Event {
	events := make([]Event, 0, 5*n+1)
	for i := range n {
		parent := fmt.Sprintf("TestCase%d", i)
		child := parent + "/sub"
		events = append(events,
			Event{Action: "run", Package: "p", Test: parent},
			Event{Action: "run", Package: "p", Test: child},
			Event{Action: "output", Package: "p", Test: child, Output: "    x_test.go:1: boom\n"},
			Event{Action: "fail", Package: "p", Test: child},
			Event{Action: "fail", Package: "p", Test: parent},
		)
	}
	return append(events, Event{Action: "fail", Package: "p"})
}

func BenchmarkBuildNestedFailures(b *testing.B) {
	for _, n := range []int{5000, 10000, 20000} {
		events := nestedFailures(n)
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			for b.Loop() {
				Build(events, DefaultOptions())
			}
		})
	}
}

func TestNestedFailuresListOnlyTheChild(t *testing.T) {
	t.Parallel()
	r := Build(nestedFailures(3), DefaultOptions())
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

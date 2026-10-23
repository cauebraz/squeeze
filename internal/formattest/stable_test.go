package formattest

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
)

type recorder struct {
	testing.TB
	failed bool
}

func (r *recorder) Helper() {}

func (r *recorder) Fatalf(string, ...any) { r.failed = true }

func TestCheckStable(t *testing.T) {
	sequence := func(results ...core.Result) func() (core.Result, error) {
		i := 0
		return func() (core.Result, error) {
			r := results[i%len(results)]
			i++
			return r, nil
		}
	}
	calls := 0
	tests := map[string]struct {
		run  func() (core.Result, error)
		fail bool
	}{
		"same":            {run: sequence(core.Result{Text: "a\n", Data: 1})},
		"same error":      {run: func() (core.Result, error) { return core.Result{}, errors.New("x") }},
		"text differs":    {run: sequence(core.Result{Text: "a\n"}, core.Result{Text: "b\n"}), fail: true},
		"data differs":    {run: sequence(core.Result{Text: "a\n", Data: 1}, core.Result{Text: "a\n", Data: 2}), fail: true},
		"envelope errors": {run: sequence(core.Result{Text: "a\n", Data: make(chan int)}, core.Result{Text: "a\n"}), fail: true},
		"error differs": {run: func() (core.Result, error) {
			calls++
			return core.Result{}, fmt.Errorf("call %d", calls)
		}, fail: true},
	}
	for name, tc := range tests {
		r := &recorder{TB: t}
		CheckStable(r, tc.run)
		if r.failed != tc.fail {
			t.Errorf("%s: failed = %v, want %v", name, r.failed, tc.fail)
		}
	}
}

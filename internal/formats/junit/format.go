package junit

import (
	"flag"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
)

type options struct {
	MaxTests  int
	MaxOutput int
}

func defaultOptions() options {
	return options{MaxTests: 30, MaxOutput: 10}
}

type Format struct{}

func (Format) Name() string { return "junit" }

func (Format) Description() string {
	return "JUnit XML test reports from pytest, go-junit-report, gotestsum, Surefire, jest-junit, checkov and others"
}

var conditionNames = []string{"error", "fail", "flaky", "skip"}

func (Format) Conditions() []string { return slices.Clone(conditionNames) }

func (Format) Detect(head []byte) bool {
	return detect(head)
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.MaxTests, "max-tests", "failed tests listed, 0 for all")
	core.CountFlag(fs, &o.MaxOutput, "max-output", "lines of each failure text, system-out and system-err, 0 for all")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	root, err := parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := build(root, o)
	return core.Result{Text: render(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r report) []string {
	errored := r.Errors > 0 || value(r.Attributes.Errors) > 0
	return core.Conditions(map[string]bool{
		"error": errored,
		"fail":  errored || r.Failures > 0 || value(r.Attributes.Failures) > 0,
		"flaky": r.Flaky > 0,
		"skip":  r.Skipped > 0 || value(r.Attributes.Skipped) > 0,
	})
}

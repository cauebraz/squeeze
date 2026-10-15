package junit

import (
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxCases int
	MaxLines int
}

func DefaultOptions() Options {
	return Options{MaxCases: 30, MaxLines: 10}
}

type Format struct{}

func (Format) Name() string { return "junit" }

func (Format) Description() string {
	return "JUnit XML test reports from pytest, go-junit-report, gotestsum, Surefire, jest-junit, checkov and others"
}

var conditionNames = []string{"error", "fail", "flaky", "skipped"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

func (Format) Detect(head []byte) bool {
	return detect(head)
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxCases, "max-cases", "failing cases listed, 0 for all")
	core.CountFlag(fs, &o.MaxLines, "max-lines", "lines of each failure text, system-out and system-err, 0 for all")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	root, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(root, o)
	return core.Result{Text: Text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	errors := r.Errors > 0 || value(r.Attributes.Errors) > 0
	return core.Conditions(map[string]bool{
		"error":   errors,
		"fail":    errors || r.Failures > 0 || value(r.Attributes.Failures) > 0,
		"flaky":   r.Flaky > 0,
		"skipped": r.Skipped > 0 || value(r.Attributes.Skipped) > 0,
	})
}

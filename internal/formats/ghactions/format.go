package ghactions

import (
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type options struct {
	MaxLines int
	All      bool
}

func defaultOptions() options {
	return options{MaxLines: 30}
}

type Format struct{}

func (Format) Name() string { return "ghactions" }

func (Format) Description() string {
	return "GitHub Actions job logs from gh run view --log or --log-failed"
}

func (Format) Conditions() []string { return core.SharedConditions() }

func (Format) Detect(head []byte) bool {
	return detect(head)
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.MaxLines, "max-lines", "output lines of the failing step, 0 for all")
	fs.BoolVar(&o.All, "all", false, "also print the output of the steps that did not fail")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	jobs, err := parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := build(jobs, o)
	return core.Result{Text: text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r report) []string {
	return core.Conditions(map[string]bool{
		"error":   r.Errors > 0,
		"fail":    r.Errors > 0,
		"warning": r.Warnings > 0,
	})
}

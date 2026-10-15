package ghactions

import (
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxLines int
	All      bool
}

func DefaultOptions() Options {
	return Options{MaxLines: 30}
}

type Format struct{}

func (Format) Name() string { return "ghactions" }

func (Format) Description() string {
	return "GitHub Actions job logs from gh run view --log or --log-failed"
}

var conditionNames = []string{"error", "fail", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

func (Format) Detect(head []byte) bool {
	return detect(head)
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxLines, "max-lines", "output lines of the failing step, 0 for all")
	fs.BoolVar(&o.All, "all", false, "also print the output of the steps that did not fail")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	jobs, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(jobs, o)
	return core.Result{Text: Text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	return core.Conditions(map[string]bool{
		"error":   r.Errors > 0,
		"fail":    r.Errors > 0,
		"warning": r.Warnings > 0,
	})
}

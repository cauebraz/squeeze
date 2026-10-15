package tsc

import (
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxErrors int
	All       bool
}

func DefaultOptions() Options {
	return Options{MaxErrors: 50}
}

type Format struct{}

func (Format) Name() string { return "tsc" }

func (Format) Description() string {
	return "TypeScript compiler diagnostics from tsc, plain or --pretty, and tsc -b"
}

var conditionNames = []string{"error", "fail", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

func (Format) Detect(head []byte) bool {
	return detect(head)
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxErrors, "max-errors", "diagnostics listed, 0 for all")
	fs.BoolVar(&o.All, "all", false, "also print the lines that are not diagnostics, such as tsc -b --verbose status lines")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	l, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(l, o)
	return core.Result{Text: Text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	return core.Conditions(map[string]bool{
		"error":   r.Errors > 0,
		"fail":    r.Errors > 0 || r.Warnings > 0 || r.Messages > 0 || r.SummaryErrors > 0,
		"warning": r.Warnings > 0,
	})
}

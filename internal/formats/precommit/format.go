package precommit

import (
	"flag"
	"slices"

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

func (Format) Name() string { return "precommit" }

func (Format) Description() string {
	return "pre-commit run output, with or without --verbose and color"
}

var conditionNames = []string{"error", "fail", "modified", "warning"}

func (Format) Conditions() []string { return slices.Clone(conditionNames) }

func (Format) Detect(head []byte) bool {
	return detect(head)
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.MaxLines, "max-lines", "output lines per failed hook, 0 for all")
	fs.BoolVar(&o.All, "all", false, "also list passed and skipped hooks with their output, and the diff of --show-diff-on-failure")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	run, err := parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := build(run, o)
	return core.Result{Text: text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r report) []string {
	return core.Conditions(map[string]bool{
		"error":    len(r.Errors) > 0,
		"fail":     r.Failed+r.Unfinished > 0 || len(r.Errors) > 0,
		"modified": r.Modified > 0,
		"warning":  len(r.Warnings) > 0,
	})
}

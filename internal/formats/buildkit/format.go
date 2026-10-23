package buildkit

import (
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type options struct {
	MaxLines int
	All      bool
}

func defaultOptions() options {
	return options{MaxLines: 20}
}

type Format struct{}

func (Format) Name() string { return "buildkit" }

func (Format) Description() string {
	return "Docker BuildKit build output from docker buildx build --progress=plain or rawjson"
}

func (Format) Conditions() []string { return core.SharedConditions() }

func (Format) Detect(head []byte) bool {
	return detect(head)
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.MaxLines, "max-lines", "log lines of each failed step, 0 for all")
	fs.BoolVar(&o.All, "all", false, "print every step with its log")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	b, err := parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := build(b, o)
	return core.Result{Text: text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r report) []string {
	failed := r.FailedSteps > 0 || len(r.Errors) > 0
	return core.Conditions(map[string]bool{
		"error":   failed,
		"fail":    failed,
		"warning": len(r.Warnings) > 0,
	})
}

package buildkit

import (
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxLines int
	All      bool
}

func DefaultOptions() Options {
	return Options{MaxLines: 20}
}

type Format struct{}

func (Format) Name() string { return "buildkit" }

func (Format) Description() string {
	return "Docker BuildKit build output from docker buildx build --progress=plain or rawjson"
}

var conditionNames = []string{"error", "fail", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

func (Format) Detect(head []byte) bool {
	return detect(head)
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxLines, "max-lines", "log lines of each failed step, 0 for all")
	fs.BoolVar(&o.All, "all", false, "print every step with its log")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	b, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(b, o)
	return core.Result{Text: Text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	failed := r.FailedSteps > 0 || len(r.Errors) > 0
	return core.Conditions(map[string]bool{
		"error":   failed,
		"fail":    failed,
		"warning": len(r.Warnings) > 0,
	})
}

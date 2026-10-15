package golangci

import (
	"bytes"
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxIssues int
}

func DefaultOptions() Options {
	return Options{MaxIssues: 50}
}

type Format struct{}

func (Format) Name() string { return "golangci" }

func (Format) Description() string {
	return "golangci-lint issues from golangci-lint run --output.json.path stdout"
}

var conditionNames = []string{"error", "fail", "fixable", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

var prefix = []byte(`{"Issues":[`)

func (Format) Detect(head []byte) bool {
	var start []byte
	for _, c := range head {
		if len(start) == len(prefix) {
			break
		}
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			start = append(start, c)
		}
	}
	if !bytes.Equal(start, prefix) {
		return false
	}
	return bytes.Contains(head, []byte(`"FromLinter"`)) || bytes.Contains(head, []byte(`"Report"`))
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxIssues, "max-issues", "issues listed, 0 for all")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	out, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(out, o)
	return core.Result{Text: Text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	return core.Conditions(map[string]bool{
		"error":   r.Errors > 0 || r.RunErrors > 0,
		"fail":    r.Total > 0 || r.RunErrors > 0,
		"fixable": r.Fixable > 0,
		"warning": r.Warnings > 0 || r.RunWarnings > 0,
	})
}

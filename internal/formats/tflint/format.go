package tflint

import (
	"bytes"
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxIssues int
	NoDetail  bool
}

func DefaultOptions() Options {
	return Options{MaxIssues: 50}
}

type Format struct{}

func (Format) Name() string { return "tflint" }

func (Format) Description() string { return "TFLint findings from tflint --format json" }

var conditionNames = []string{"error", "fail", "fixable", "notice", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

func (Format) Detect(head []byte) bool {
	trimmed := bytes.TrimLeft(head, " \t\r\n")
	if !bytes.HasPrefix(trimmed, []byte(`{"issues":`)) && !bytes.HasPrefix(trimmed, []byte(`{"issues" :`)) {
		return false
	}
	return bytes.Contains(head, []byte(`"errors":`)) || bytes.Contains(head, []byte(`"errors" :`))
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxIssues, "max-issues", "issues listed, 0 for all")
	fs.BoolVar(&o.NoDetail, "no-detail", false, "print the issue, its location and rule only")
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
	return core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	return core.Conditions(map[string]bool{
		"error":   r.Errors > 0 || r.ToolErrors > 0,
		"fail":    r.Errors+r.Warnings+r.Notices+r.ToolErrors > 0,
		"fixable": r.Fixable > 0,
		"notice":  r.Notices > 0,
		"warning": r.Warnings > 0,
	})
}

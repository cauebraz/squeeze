package tflint

import (
	"bytes"
	"flag"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type options struct {
	MaxIssues int
	NoDetail  bool
}

func defaultOptions() options {
	return options{MaxIssues: 50}
}

type Format struct{}

func (Format) Name() string { return "tflint" }

func (Format) Description() string { return "TFLint findings from tflint --format json" }

var conditionNames = []string{"error", "fail", "fixable", "notice", "warning"}

func (Format) Conditions() []string { return slices.Clone(conditionNames) }

func (Format) Detect(head []byte) bool {
	trimmed := bytes.TrimLeft(head, " \t\r\n")
	if !bytes.HasPrefix(trimmed, []byte(`{"issues":`)) && !bytes.HasPrefix(trimmed, []byte(`{"issues" :`)) {
		return false
	}
	return bytes.Contains(head, []byte(`"errors":`)) || bytes.Contains(head, []byte(`"errors" :`))
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.MaxIssues, "max-issues", "issues listed, 0 for all")
	fs.BoolVar(&o.NoDetail, "no-detail", false, "print the issue, its location and rule only")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	out, err := parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := build(out, o)
	return core.Result{Text: text(r, o), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r report) []string {
	return core.Conditions(map[string]bool{
		"error":   r.Errors > 0 || r.ToolErrors > 0,
		"fail":    r.Errors+r.Warnings+r.Notices+r.ToolErrors > 0,
		"fixable": r.Fixable > 0,
		"notice":  r.Notices > 0,
		"warning": r.Warnings > 0 || slices.ContainsFunc(r.Problems, func(d tfdiag.Diagnostic) bool { return d.Severity == "warning" }),
	})
}

package checkov

import (
	"bytes"
	"flag"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type options struct {
	maxChecks int
	all       bool
}

func defaultOptions() options {
	return options{maxChecks: 50}
}

type Format struct{}

func (Format) Name() string { return "checkov" }

func (Format) Description() string { return "Checkov results from checkov -o json" }

var conditionNames = []string{"critical", "error", "fail", "high", "skip", "warning"}

func (Format) Conditions() []string { return slices.Clone(conditionNames) }

func (Format) Detect(head []byte) bool {
	trimmed := bytes.TrimLeft(head, " \t\r\n")
	if len(trimmed) == 0 || (trimmed[0] != '{' && trimmed[0] != '[') {
		return false
	}
	if bytes.Contains(head, []byte(`"check_type"`)) && bytes.Contains(head, []byte(`"results"`)) {
		return true
	}
	return trimmed[0] == '{' && bytes.Contains(head, []byte(`"checkov_version"`)) && bytes.Contains(head, []byte(`"resource_count"`))
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.maxChecks, "max-checks", "failed checks listed, 0 for all")
	fs.BoolVar(&o.all, "all", false, "also list skipped checks with their suppression comment")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	in, err := parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := build(in, o)
	return core.Result{Text: text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r report) []string {
	errs, warns := tfdiag.Count(r.Diagnostics)
	return core.Conditions(map[string]bool{
		"critical": r.BySeverity["CRITICAL"] > 0,
		"error":    r.ParsingErrors+errs > 0,
		"fail":     r.Failed > 0,
		"high":     r.BySeverity["HIGH"]+r.BySeverity["CRITICAL"] > 0,
		"skip":     r.Skipped > 0,
		"warning":  warns > 0,
	})
}

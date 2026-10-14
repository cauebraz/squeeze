package checkov

import (
	"bytes"
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxChecks int
	All       bool
}

func DefaultOptions() Options {
	return Options{MaxChecks: 50}
}

type Format struct{}

func (Format) Name() string { return "checkov" }

func (Format) Description() string { return "Checkov results from checkov -o json" }

var conditionNames = []string{"critical", "error", "fail", "high", "skip"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

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
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxChecks, "max-checks", "failed checks listed, 0 for all")
	fs.BoolVar(&o.All, "all", false, "also list skipped checks with their suppression comment")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	reports, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(reports, o)
	return core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	return core.Conditions(map[string]bool{
		"critical": r.BySeverity["CRITICAL"] > 0,
		"error":    r.ParsingErrors > 0,
		"fail":     r.Failed > 0,
		"high":     r.BySeverity["HIGH"]+r.BySeverity["CRITICAL"] > 0,
		"skip":     r.Skipped > 0,
	})
}

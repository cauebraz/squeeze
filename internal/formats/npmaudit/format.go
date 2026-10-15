package npmaudit

import (
	"bytes"
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxVulnerabilities int
}

func DefaultOptions() Options {
	return Options{MaxVulnerabilities: 50}
}

type Format struct{}

func (Format) Name() string { return "npmaudit" }

func (Format) Description() string { return "npm vulnerabilities from npm audit --json" }

var conditionNames = []string{"critical", "fail", "fixable", "high", "info", "low", "moderate"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

var prefix = []byte(`{"auditReportVersion":`)

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
	return bytes.Equal(start, prefix) && bytes.Contains(head, []byte(`"vulnerabilities"`))
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxVulnerabilities, "max-vulnerabilities", "vulnerable packages listed, 0 for all")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	a, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(a, o)
	return core.Result{Text: Text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	at := func(level string) bool {
		for _, c := range r.BySeverity {
			if rank(c.Name) >= rank(level) {
				return true
			}
		}
		return false
	}
	return core.Conditions(map[string]bool{
		"critical": at("critical"),
		"fail":     at("low"),
		"fixable":  r.Fixes.Auto+r.Fixes.Force+r.Fixes.Breaking > 0,
		"high":     at("high"),
		"info":     at("info"),
		"low":      at("low"),
		"moderate": at("moderate"),
	})
}

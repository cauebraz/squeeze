package npmaudit

import (
	"bytes"
	"flag"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/logscan"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type options struct {
	maxVulnerabilities int
}

func defaultOptions() options {
	return options{maxVulnerabilities: 50}
}

type Format struct{}

func (Format) Name() string { return "npmaudit" }

func (Format) Description() string { return "npm vulnerabilities from npm audit --json" }

var conditionNames = []string{"critical", "error", "fail", "fixable", "high", "info", "low", "moderate", "warning"}

func (Format) Conditions() []string { return slices.Clone(conditionNames) }

var prefix = []byte(`{"auditReportVersion":`)

func (Format) Detect(head []byte) bool {
	_, rest := logscan.LeadingLines(head)
	return core.HasJSONPrefix(rest, prefix) && bytes.Contains(rest, []byte(`"vulnerabilities"`))
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.maxVulnerabilities, "max-vulnerabilities", "vulnerable packages listed, 0 for all")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	a, err := parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := build(a, o)
	return core.Result{Text: text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r report) []string {
	at := func(level string) bool {
		for _, c := range r.BySeverity {
			if severities.AtLeast(c.Name, level) {
				return true
			}
		}
		return false
	}
	errs, warns := tfdiag.Count(r.Diagnostics)
	return core.Conditions(map[string]bool{
		"critical": at("critical"),
		"error":    errs > 0,
		"warning":  warns > 0,
		"fail":     at("low"),
		"fixable":  r.Fixes.Auto+r.Fixes.Force+r.Fixes.Breaking > 0,
		"high":     at("high"),
		"info":     at("info"),
		"low":      at("low"),
		"moderate": at("moderate"),
	})
}

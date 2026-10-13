package tflint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const maxRulesInNotice = 5

type RuleCount struct {
	Rule  string `json:"rule"`
	Count int    `json:"count"`
}

type Report struct {
	Tool               string              `json:"tool"`
	Errors             int                 `json:"errors"`
	Warnings           int                 `json:"warnings"`
	Notices            int                 `json:"notices"`
	ToolErrors         int                 `json:"tool_errors"`
	Fixable            int                 `json:"fixable"`
	Fixed              int                 `json:"fixed"`
	Rules              []RuleCount         `json:"rules"`
	Diagnostics        []tfdiag.Diagnostic `json:"diagnostics"`
	DiagnosticsOmitted int                 `json:"diagnostics_omitted"`
	OmittedRules       []RuleCount         `json:"omitted_rules"`
}

func severity(s string) string {
	switch s {
	case "warning":
		return "warning"
	case "info", "notice":
		return "notice"
	default:
		return "error"
	}
}

func location(r Range) (string, int, int) {
	return core.Clean(r.Filename), r.Start.Line, r.Start.Column
}

func where(r Range) string {
	file, line, col := location(r)
	if line > 0 {
		return fmt.Sprintf("%s:%d:%d", file, line, col)
	}
	return file
}

func Build(out *Output, o Options) Report {
	r := Report{Tool: "tflint", Rules: []RuleCount{}, Diagnostics: []tfdiag.Diagnostic{}, OmittedRules: []RuleCount{}}
	var ds []tfdiag.Diagnostic
	for _, e := range *out.Errors {
		d := tfdiag.Diagnostic{Severity: severity(e.Severity), Summary: core.Clean(e.Summary), Detail: strings.TrimSpace(e.Message), Address: "tflint"}
		if d.Summary == "" {
			d.Summary, d.Detail = core.Clean(e.Message), ""
		}
		if e.Range != nil {
			d.File, d.Line, d.Column = location(*e.Range)
		}
		if d.Severity == "notice" {
			d.Severity = "warning"
		}
		r.ToolErrors++
		ds = append(ds, d)
	}
	counts := map[string]int{}
	for _, is := range *out.Issues {
		name := core.Clean(is.Rule.Name)
		d := tfdiag.Diagnostic{Severity: severity(is.Rule.Severity), Summary: core.Clean(is.Message), Address: name}
		d.File, d.Line, d.Column = location(is.Range)
		switch {
		case is.Fixed:
			d.Address += ", fixed"
			r.Fixed++
		case is.Fixable:
			d.Address += ", fixable"
			r.Fixable++
		}
		if len(is.Callers) > 0 {
			chain := make([]string, 0, len(is.Callers))
			for _, c := range is.Callers {
				chain = append(chain, where(c))
			}
			d.Detail = "called from " + strings.Join(chain, " <- ")
		}
		switch d.Severity {
		case "error":
			r.Errors++
		case "warning":
			r.Warnings++
		case "notice":
			r.Notices++
		}
		counts[name]++
		ds = append(ds, d)
	}
	for name, n := range counts {
		r.Rules = append(r.Rules, RuleCount{Rule: name, Count: n})
	}
	sortRules(r.Rules)
	ds = tfdiag.Normalize(ds)
	kept, omitted := core.Limit(ds, o.MaxIssues)
	r.Diagnostics = append(r.Diagnostics, kept...)
	r.DiagnosticsOmitted = omitted
	if omitted > 0 {
		cut := map[string]int{}
		for _, d := range ds[len(kept):] {
			cut[ruleOf(d)] += max(d.Count, 1)
		}
		for name, n := range cut {
			r.OmittedRules = append(r.OmittedRules, RuleCount{Rule: name, Count: n})
		}
		sortRules(r.OmittedRules)
	}
	return r
}

func ruleOf(d tfdiag.Diagnostic) string {
	name, _, _ := strings.Cut(d.Address, ", ")
	return name
}

func sortRules(rs []RuleCount) {
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].Count != rs[j].Count {
			return rs[i].Count > rs[j].Count
		}
		return rs[i].Rule < rs[j].Rule
	})
}

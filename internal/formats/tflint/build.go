package tflint

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const maxRulesInNotice = 5

type Issue struct {
	tfdiag.Diagnostic
	Rule    string `json:"rule"`
	Fixable bool   `json:"fixable,omitempty"`
	Fixed   bool   `json:"fixed,omitempty"`
}

type Report struct {
	Tool          string              `json:"tool"`
	ToolVersion   string              `json:"tool_version"`
	Errors        int                 `json:"errors"`
	Warnings      int                 `json:"warnings"`
	Notices       int                 `json:"notices"`
	ToolErrors    int                 `json:"tool_errors"`
	Fixable       int                 `json:"fixable"`
	Fixed         int                 `json:"fixed"`
	Rules         []core.Count        `json:"rules"`
	Problems      []tfdiag.Diagnostic `json:"problems"`
	Issues        []Issue             `json:"issues"`
	IssuesOmitted int                 `json:"issues_omitted"`
	OmittedByRule []core.Count        `json:"omitted_by_rule"`
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
	r := Report{Tool: "tflint", Rules: []core.Count{}, Problems: []tfdiag.Diagnostic{}, Issues: []Issue{}, OmittedByRule: []core.Count{}}
	var problems []tfdiag.Diagnostic
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
		problems = append(problems, d)
	}
	r.Problems = append(r.Problems, tfdiag.Normalize(problems)...)
	counts := map[string]int{}
	var issues []Issue
	for _, is := range *out.Issues {
		name := core.Clean(is.Rule.Name)
		d := tfdiag.Diagnostic{Severity: severity(is.Rule.Severity), Summary: core.Clean(is.Message), Address: name}
		d.File, d.Line, d.Column = location(is.Range)
		if chain := callers(is); len(chain) > 0 {
			d.Detail = "evaluated in " + strings.Join(chain, " -> ")
		}
		switch d.Severity {
		case "error":
			r.Errors++
		case "warning":
			r.Warnings++
		case "notice":
			r.Notices++
		}
		if is.Fixed {
			r.Fixed++
		} else if is.Fixable {
			r.Fixable++
		}
		counts[name]++
		issues = append(issues, Issue{Diagnostic: d, Rule: name, Fixable: is.Fixable && !is.Fixed, Fixed: is.Fixed})
	}
	r.Rules = core.SortedCounts(counts)
	issues = fold(issues)
	kept, _ := core.Limit(issues, o.MaxIssues)
	r.Issues = append(r.Issues, kept...)
	cut := map[string]int{}
	for _, is := range issues[len(kept):] {
		cut[is.Rule] += max(is.Count, 1)
		r.IssuesOmitted += max(is.Count, 1)
	}
	r.OmittedByRule = core.SortedCounts(cut)
	return r
}

func callers(is RawIssue) []string {
	issueAt := where(is.Range)
	var chain []string
	for i, c := range is.Callers {
		at := where(c)
		if i == 0 && at == issueAt {
			continue
		}
		chain = append(chain, at)
	}
	return chain
}

func fold(issues []Issue) []Issue {
	var ds []tfdiag.Diagnostic
	meta := map[string][2]bool{}
	for _, is := range issues {
		ds = append(ds, is.Diagnostic)
		meta[tfdiag.Key(is.Diagnostic)] = [2]bool{is.Fixable, is.Fixed}
	}
	out := make([]Issue, 0, len(ds))
	for _, d := range tfdiag.Normalize(ds) {
		m := meta[tfdiag.Key(d)]
		out = append(out, Issue{Diagnostic: d, Rule: d.Address, Fixable: m[0], Fixed: m[1]})
	}
	return out
}

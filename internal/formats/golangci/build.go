package golangci

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const (
	maxLintersInNotice = 5
	typecheck          = "typecheck"
)

type Problem struct {
	tfdiag.Diagnostic
	Linter  string `json:"linter"`
	Fixable bool   `json:"fixable,omitempty"`
}

type Report struct {
	Tool            string              `json:"tool"`
	ToolVersion     string              `json:"tool_version"`
	Total           int                 `json:"total"`
	Errors          int                 `json:"errors"`
	Warnings        int                 `json:"warnings"`
	Severities      []core.Count        `json:"severities"`
	Linters         []core.Count        `json:"linters"`
	EnabledLinters  []string            `json:"enabled_linters"`
	Fixable         int                 `json:"fixable"`
	RunErrors       int                 `json:"run_errors"`
	RunWarnings     int                 `json:"run_warnings"`
	Run             []tfdiag.Diagnostic `json:"run_diagnostics"`
	Issues          []Problem           `json:"issues"`
	IssuesOmitted   int                 `json:"issues_omitted"`
	OmittedByLinter []core.Count        `json:"omitted_by_linter"`
}

var compileLine = regexp.MustCompile(`^(\S+\.go):(\d+)(?::(\d+))?: (.+)$`)

func severity(s string) string {
	s = strings.ToLower(core.Clean(s))
	if s == "" {
		return "error"
	}
	return s
}

func Build(out *Output, o Options) Report {
	r := Report{Tool: "golangci-lint", Severities: []core.Count{}, Linters: []core.Count{}, EnabledLinters: []string{}, Run: []tfdiag.Diagnostic{}, Issues: []Problem{}, OmittedByLinter: []core.Count{}}
	for _, l := range out.Report.Linters {
		if l.Enabled {
			r.EnabledLinters = append(r.EnabledLinters, core.Clean(l.Name))
		}
	}
	r.Run = tfdiag.Normalize(run(out))
	for _, d := range r.Run {
		if d.Severity == "error" {
			r.RunErrors += max(d.Count, 1)
		} else {
			r.RunWarnings += max(d.Count, 1)
		}
	}
	var checks, problems []Problem
	for _, is := range out.Issues {
		linter := core.Clean(is.FromLinter)
		base := tfdiag.Diagnostic{Severity: severity(is.Severity), Address: linter, File: core.Clean(is.Pos.Filename), Line: is.Pos.Line, Column: is.Pos.Column}
		fixable := len(is.SuggestedFixes) > 0 || is.Replacement != nil
		ds := []tfdiag.Diagnostic{base}
		if linter == typecheck {
			ds = expand(is.Text, base)
		} else {
			ds[0].Summary = core.Clean(is.Text)
		}
		for _, d := range ds {
			p := Problem{Diagnostic: d, Linter: linter, Fixable: fixable}
			if linter == typecheck {
				checks = append(checks, p)
			} else {
				problems = append(problems, p)
			}
		}
	}
	severities, linters := map[string]int{}, map[string]int{}
	for _, p := range append(checks, problems...) {
		r.Total++
		severities[p.Severity]++
		linters[p.Linter]++
		if p.Fixable {
			r.Fixable++
		}
	}
	r.Errors, r.Warnings = severities["error"], severities["warning"]
	r.Severities = core.SortedCounts(severities)
	r.Linters = core.SortedCounts(linters)
	r.Issues = append(r.Issues, fold(checks)...)
	problems = fold(problems)
	kept, _ := core.Limit(problems, o.MaxIssues)
	r.Issues = append(r.Issues, kept...)
	cut := map[string]int{}
	for _, p := range problems[len(kept):] {
		name := p.Linter
		if name == "" {
			name = "no linter"
		}
		cut[name] += max(p.Count, 1)
		r.IssuesOmitted += max(p.Count, 1)
	}
	r.OmittedByLinter = core.SortedCounts(cut)
	return r
}

func run(out *Output) []tfdiag.Diagnostic {
	var ds []tfdiag.Diagnostic
	if e := core.Clean(out.Report.Error); e != "" {
		ds = append(ds, tfdiag.Diagnostic{Severity: "error", Summary: e})
	}
	for _, w := range out.Report.Warnings {
		ds = append(ds, tfdiag.Diagnostic{Severity: "warning", Summary: core.Clean(w.Text), Address: core.Clean(w.Tag)})
	}
	for _, line := range out.Trailing {
		d := tfdiag.Diagnostic{Severity: "warning", Summary: core.Clean(line)}
		for _, level := range []string{"level=error", "level=fatal", "level=panic"} {
			if strings.HasPrefix(line, level) {
				d.Severity = "error"
			}
		}
		ds = append(ds, d)
	}
	return ds
}

func expand(text string, base tfdiag.Diagnostic) []tfdiag.Diagnostic {
	var out []tfdiag.Diagnostic
	var rest []string
	for _, line := range strings.Split(text, "\n") {
		m := compileLine.FindStringSubmatch(line)
		if m == nil {
			line = strings.TrimSpace(strings.TrimPrefix(core.Clean(line), ":"))
			switch {
			case line == "" || strings.HasPrefix(line, "# "):
			case len(out) > 0:
				out[len(out)-1].Summary += " " + line
			default:
				rest = append(rest, line)
			}
			continue
		}
		d := base
		d.File = core.Clean(m[1])
		d.Line, _ = strconv.Atoi(m[2])
		d.Column, _ = strconv.Atoi(m[3])
		d.Summary = core.Clean(m[4])
		out = append(out, d)
	}
	if len(out) == 0 || len(rest) > 0 {
		base.Summary = strings.Join(rest, " ")
		if base.Summary == "" {
			base.Summary = strings.TrimSpace(strings.TrimPrefix(core.Clean(text), ":"))
		}
		out = append([]tfdiag.Diagnostic{base}, out...)
	}
	return out
}

func fold(problems []Problem) []Problem {
	ds := make([]tfdiag.Diagnostic, 0, len(problems))
	fixable := map[string]bool{}
	for _, p := range problems {
		ds = append(ds, p.Diagnostic)
		fixable[tfdiag.Key(p.Diagnostic)] = fixable[tfdiag.Key(p.Diagnostic)] || p.Fixable
	}
	out := make([]Problem, 0, len(ds))
	for _, d := range tfdiag.Normalize(ds) {
		out = append(out, Problem{Diagnostic: d, Linter: d.Address, Fixable: fixable[tfdiag.Key(d)]})
	}
	return out
}

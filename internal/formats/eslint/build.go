package eslint

import (
	"path"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const maxRulesInNotice = 5

type Problem struct {
	tfdiag.Diagnostic
	Rule    string `json:"rule"`
	Fixable bool   `json:"fixable,omitempty"`
}

type Report struct {
	Tool             string              `json:"tool"`
	ToolVersion      string              `json:"tool_version"`
	Root             string              `json:"root"`
	Files            int                 `json:"files"`
	FilesWithProblem int                 `json:"files_with_problems"`
	Errors           int                 `json:"errors"`
	Warnings         int                 `json:"warnings"`
	Fatal            int                 `json:"fatal"`
	Fixable          int                 `json:"fixable"`
	Suppressed       int                 `json:"suppressed"`
	Rules            []core.Count        `json:"rules"`
	ParseErrors      []tfdiag.Diagnostic `json:"parse_errors"`
	Problems         []Problem           `json:"problems"`
	ProblemsOmitted  int                 `json:"problems_omitted"`
	OmittedByRule    []core.Count        `json:"omitted_by_rule"`
}

func severity(s int) string {
	if s == 1 {
		return "warning"
	}
	return "error"
}

func root(out *Output) string {
	if out.Cwd != "" {
		return strings.TrimSuffix(out.Cwd, "/")
	}
	common := ""
	for i, r := range out.Results {
		dir := path.Dir(r.FilePath)
		if i == 0 {
			common = dir
			continue
		}
		for common != dir && !strings.HasPrefix(dir, common+"/") && common != "/" && common != "." {
			common = path.Dir(common)
		}
	}
	if common == "." || common == "/" {
		return ""
	}
	return common
}

func relative(file, root string) string {
	if root != "" && strings.HasPrefix(file, root+"/") {
		return file[len(root)+1:]
	}
	return file
}

func Build(out *Output, o Options) Report {
	r := Report{Tool: "eslint", Rules: []core.Count{}, ParseErrors: []tfdiag.Diagnostic{}, Problems: []Problem{}, OmittedByRule: []core.Count{}}
	base := root(out)
	r.Root = core.Clean(base)
	counts := map[string]int{}
	var fatal []tfdiag.Diagnostic
	var problems []Problem
	for _, res := range out.Results {
		r.Files++
		r.Errors += res.ErrorCount
		r.Warnings += res.WarningCount
		r.Fatal += res.FatalErrorCount
		r.Fixable += res.FixableErrorCount + res.FixableWarningCount
		r.Suppressed += len(res.SuppressedMessages)
		if len(*res.Messages) > 0 {
			r.FilesWithProblem++
		}
		file := core.Clean(relative(res.FilePath, base))
		for _, m := range *res.Messages {
			d := tfdiag.Diagnostic{Severity: severity(m.Severity), Summary: core.Clean(m.Message), File: file, Line: m.Line, Column: m.Column}
			if m.Fatal {
				fatal = append(fatal, d)
				continue
			}
			if m.RuleID != nil {
				d.Address = core.Clean(*m.RuleID)
			}
			counts[d.Address]++
			problems = append(problems, Problem{Diagnostic: d, Rule: d.Address, Fixable: m.Fix != nil})
		}
	}
	delete(counts, "")
	r.Rules = core.SortedCounts(counts)
	r.ParseErrors = append(r.ParseErrors, tfdiag.Normalize(fatal)...)
	problems = fold(problems)
	kept, _ := core.Limit(problems, o.MaxProblems)
	r.Problems = append(r.Problems, kept...)
	cut := map[string]int{}
	for _, p := range problems[len(kept):] {
		name := p.Rule
		if name == "" {
			name = "no rule"
		}
		cut[name] += max(p.Count, 1)
		r.ProblemsOmitted += max(p.Count, 1)
	}
	r.OmittedByRule = core.SortedCounts(cut)
	return r
}

func fold(problems []Problem) []Problem {
	ds := make([]tfdiag.Diagnostic, 0, len(problems))
	fixable := map[string]bool{}
	for _, p := range problems {
		ds = append(ds, p.Diagnostic)
		fixable[tfdiag.Key(p.Diagnostic)] = p.Fixable
	}
	out := make([]Problem, 0, len(ds))
	for _, d := range tfdiag.Normalize(ds) {
		out = append(out, Problem{Diagnostic: d, Rule: d.Address, Fixable: fixable[tfdiag.Key(d)]})
	}
	return out
}

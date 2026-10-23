package pytest

import (
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

func Build(input []byte, o Options) (Report, bool) {
	p := parser{r: Report{
		Tool:         "pytest",
		Failures:     []Failure{},
		Warnings:     []Warning{},
		Notes:        []string{},
		Outcomes:     []string{},
		Sections:     map[string]int{},
		sectionLines: map[string][]string{},
	}}
	p.r.Summary.Other = map[string]int{}
	for raw := range core.SplitLines(string(input)) {
		p.line(core.Line(raw))
	}
	p.closeEntry()
	if !p.found {
		return Report{}, false
	}
	p.finish(o)
	return p.r, true
}

func (p *parser) finish(o Options) {
	var failures []Failure
	for _, e := range p.entries {
		f := p.failure(e, o)
		if f.Test == "" && len(f.Lines) == 0 {
			continue
		}
		failures = append(failures, f)
	}
	for _, rec := range p.records {
		if rec.used {
			continue
		}
		kind := "failed"
		if rec.word == "ERROR" {
			kind = "error"
		}
		failures = append(failures, Failure{Kind: kind, Test: rec.test, Subtest: rec.label, Message: rec.message, Lines: []string{}, Untracked: true})
	}
	if !o.NoFold {
		failures, p.r.FailuresFolded = foldFailures(failures)
	}
	failures, p.r.FailuresOmitted = core.Limit(failures, o.MaxTests)
	if failures != nil {
		p.r.Failures = failures
	}
	p.r.Warnings = p.warnings.result()
	p.r.Warnings, p.r.WarningsOmitted = core.Limit(p.r.Warnings, o.MaxWarnings)
	p.r.Notes, p.r.NotesOmitted = core.LimitTail(p.r.Notes, o.MaxOutput)
}

func (p *parser) failure(e *entry, o Options) Failure {
	f := Failure{Kind: "failed", Test: e.title}
	if e.section == "ERRORS" {
		f.Kind = "error"
		switch {
		case strings.HasPrefix(e.title, "ERROR collecting "):
			f.When, f.Test = "collection", strings.TrimPrefix(e.title, "ERROR collecting ")
		case strings.HasPrefix(e.title, "ERROR at "):
			rest := strings.TrimPrefix(e.title, "ERROR at ")
			if when, name, ok := strings.Cut(rest, " of "); ok {
				f.When, f.Test = when, name
			}
		}
	}
	if rec := p.match(f); rec != nil {
		f.Test, f.Subtest, f.Message = rec.test, rec.label, rec.message
	}
	f.Lines, f.Hidden = selectLines(e.body, o.Full)
	f.Location = lastLocation(f.Lines)
	f.Lines, f.Omitted = core.LimitTail(f.Lines, o.MaxOutput)
	for _, c := range e.captured {
		kept, omitted := core.LimitTail(nonNil(c.lines), o.MaxOutput)
		f.Captured = append(f.Captured, Captured{Name: c.name, Lines: kept, Omitted: omitted})
	}
	return f
}

func lastLocation(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		if m := location.FindStringSubmatch(lines[i]); m != nil {
			return strings.TrimSuffix(strings.TrimSuffix(lines[i], m[1]), ":")
		}
	}
	return ""
}

func (p *parser) match(f Failure) *record {
	word := "FAILED"
	if f.Kind == "error" {
		word = "ERROR"
	}
	for _, rec := range p.records {
		if rec.used || rec.word != word {
			continue
		}
		short := domain(rec.test)
		if rec.label != "" {
			short += " " + rec.label
		}
		if f.Test == "" || rec.test == f.Test || short == f.Test {
			rec.used = true
			return rec
		}
	}
	return nil
}

func domain(nodeid string) string {
	params := ""
	if i := strings.Index(nodeid, "["); i >= 0 {
		nodeid, params = nodeid[:i], nodeid[i:]
	}
	_, rest, ok := strings.Cut(nodeid, "::")
	if !ok {
		return nodeid + params
	}
	return strings.ReplaceAll(rest, "::", ".") + params
}

func selectLines(body []string, full bool) ([]string, int) {
	var all []string
	for _, l := range body {
		if strings.TrimSpace(l) != "" && !frameSep.MatchString(l) {
			all = append(all, l)
		}
	}
	if full {
		return nonNil(all), 0
	}
	keep := make([]bool, len(all))
	marked := false
	for i, l := range all {
		switch {
		case isE(l), strings.HasPrefix(l, ">"), location.MatchString(l):
			keep[i], marked = true, true
		case i > 0 && inFunction.MatchString(all[i-1]) && strings.HasPrefix(l, " "):
			keep[i] = true
		}
	}
	if !marked {
		return nonNil(all), 0
	}
	var out []string
	hidden := 0
	for i, l := range all {
		if !keep[i] {
			hidden++
			continue
		}
		if isE(l) && strings.TrimSpace(l[1:]) == "" {
			continue
		}
		out = append(out, l)
	}
	return nonNil(normalize(out)), hidden
}

func isE(l string) bool {
	return l == "E" || strings.HasPrefix(l, "E ")
}

func normalize(lines []string) []string {
	indent := -1
	for _, l := range lines {
		if isE(l) {
			rest := l[1:]
			n := len(rest) - len(strings.TrimLeft(rest, " "))
			if indent < 0 || n < indent {
				indent = n
			}
		}
	}
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		switch {
		case isE(l):
			out = append(out, "E "+l[1+indent:])
		case strings.HasPrefix(l, ">"):
			out = append(out, "> "+strings.TrimSpace(l[1:]))
		default:
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

package pytest

import (
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/sanitize"
)

func build(input []byte, o options) (report, bool) {
	p := parser{r: report{
		Tool:              "pytest",
		Failures:          []failure{},
		WarningMessages:   []warningMessage{},
		Notes:             []string{},
		Outcomes:          []string{},
		OtherSections:     map[string]int{},
		OtherSectionLines: []sectionLines{},
	}}
	p.r.Summary.Other = map[string]int{}
	for raw := range core.SplitLines(string(input)) {
		p.line(sanitize.Line(raw))
	}
	p.closeEntry()
	if !p.found {
		return report{}, false
	}
	p.finish(o)
	return p.r, true
}

func (p *parser) finish(o options) {
	p.index = indexRecords(p.records)
	var failures []failure
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
		failures = append(failures, failure{Kind: rec.kind, Test: rec.test, Subtest: rec.label, Message: rec.message, Block: core.HeadBlock(nil, 0), FromSummaryOnly: true})
	}
	if !o.NoFold {
		failures, p.r.FailuresFolded = foldFailures(failures)
	}
	failures, p.r.TestsOmitted = core.Limit(failures, o.MaxTests)
	if failures != nil {
		p.r.Failures = failures
	}
	p.r.WarningMessages = p.warnings.result()
	p.r.WarningMessages, p.r.WarningsOmitted = core.Limit(p.r.WarningMessages, o.MaxWarnings)
	p.r.Notes, p.r.NotesOmitted = core.LimitTail(p.r.Notes, o.MaxOutput)
}

func (p *parser) failure(e *entry, o options) failure {
	f := failure{Kind: kindFailed, Test: e.title}
	if e.section == "ERRORS" {
		f.Kind = kindError
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
	if rec := p.index.match(f); rec != nil {
		f.Test, f.Subtest, f.Message = rec.test, rec.label, rec.message
	}
	lines, hidden := selectLines(e.body, o.Full)
	f.Hidden, f.Location = hidden, lastLocation(lines)
	f.Block = core.TailBlock(lines, o.MaxOutput)
	for _, c := range e.captured {
		f.Captured = append(f.Captured, capturedSection{Name: c.name, Block: core.TailBlock(c.lines, o.MaxOutput)})
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

type recordName struct {
	kind failureKind
	name string
}

type recordQueue struct {
	records []*record
	next    int
}

func (q *recordQueue) first() *record {
	if q == nil {
		return nil
	}
	for q.next < len(q.records) && q.records[q.next].used {
		q.next++
	}
	if q.next == len(q.records) {
		return nil
	}
	return q.records[q.next]
}

type recordIndex struct {
	byKind map[failureKind]*recordQueue
	byName map[recordName]*recordQueue
}

func indexRecords(records []*record) recordIndex {
	idx := recordIndex{byKind: map[failureKind]*recordQueue{}, byName: map[recordName]*recordQueue{}}
	push := func(m map[recordName]*recordQueue, key recordName, rec *record) {
		q := m[key]
		if q == nil {
			q = &recordQueue{}
			m[key] = q
		}
		if n := len(q.records); n == 0 || q.records[n-1] != rec {
			q.records = append(q.records, rec)
		}
	}
	for _, rec := range records {
		q := idx.byKind[rec.kind]
		if q == nil {
			q = &recordQueue{}
			idx.byKind[rec.kind] = q
		}
		q.records = append(q.records, rec)
		short := domain(rec.test)
		if rec.label != "" {
			short += " " + rec.label
		}
		push(idx.byName, recordName{rec.kind, rec.test}, rec)
		push(idx.byName, recordName{rec.kind, short}, rec)
	}
	return idx
}

func (idx recordIndex) match(f failure) *record {
	q := idx.byName[recordName{f.Kind, f.Test}]
	if f.Test == "" {
		q = idx.byKind[f.Kind]
	}
	rec := q.first()
	if rec != nil {
		rec.used = true
	}
	return rec
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

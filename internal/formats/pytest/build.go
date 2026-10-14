package pytest

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

var (
	sessionStart  = regexp.MustCompile(`^=+ test session starts =+$`)
	sectionLine   = regexp.MustCompile(`^=+ (.+?) =+$`)
	knownSection  = regexp.MustCompile(`^=+ (FAILURES|ERRORS|short test summary info|warnings summary(?: \(final\))?) =+$`)
	statsLine     = regexp.MustCompile(`^(?:=+ )?(no tests ran|\d+ [a-z]+(?: [a-z]+)?(?:, \d+ [a-z]+(?: [a-z]+)?)*) in (\d+(?:\.\d+)?s(?: \(\d+:\d+:\d+\))?)(?: =+)?$`)
	statPart      = regexp.MustCompile(`^(\d+) (.+)$`)
	bangLine      = regexp.MustCompile(`^!{3,} (.+?) !{3,}$`)
	entryTitle    = regexp.MustCompile(`^_{3,} (.+?) _{3,}$`)
	capturedTitle = regexp.MustCompile(`^-{3,} (.+?) -{3,}$`)
	frameSep      = regexp.MustCompile(`^(_ )+_?$`)
	location      = regexp.MustCompile(`^[^\s:][^:]*:\d+:( .*)?$`)
	crashLine     = regexp.MustCompile(`^[^\s:][^:]*:\d+: \S.*$`)
	inFunction    = regexp.MustCompile(`^[^\s:][^:]*:\d+: in \S+$`)
	progressLine  = regexp.MustCompile(`^(\S+ )?[.FsxXEPRu]+(\s+\[\s*\d+%\])?$`)
	verboseLine   = regexp.MustCompile(`^(\[gw\d+\] .*|\S+::\S.* (PASSED|FAILED|SKIPPED|XFAIL|XPASS|ERROR)( \(.*\))?(\s+\[\s*\d+%\])?)$`)
	headerLine    = regexp.MustCompile(`^(platform |rootdir: |configfile: |inifile: |testpaths: |plugins: |cachedir: |collecting|collected \d+|created: \d+|\d+ workers? \[|scheduling tests via |hypothesis profile |Using --randomly-seed)`)
	summaryEntry  = regexp.MustCompile(`^(SUB)?(FAILED|ERROR|XFAIL|XPASS|SKIPPED|PASSED|DESELECTED)(\[[^\]]*\](?: \([^)]*\))?)?(?: \[\d+\])? (.+)$`)
	versionField  = regexp.MustCompile(`pytest-([0-9][^,\s]*)`)
	locationCount = regexp.MustCompile(`^\S+: (\d+) warnings?$`)
)

var outcomeWords = map[string]bool{
	"failed": true, "passed": true, "skipped": true, "deselected": true,
	"xfailed": true, "xpassed": true, "error": true, "errors": true,
}

func knownStats(parts string) bool {
	if parts == "no tests ran" {
		return true
	}
	for _, part := range strings.Split(parts, ", ") {
		if m := statPart.FindStringSubmatch(part); m != nil && outcomeWords[m[2]] {
			return true
		}
	}
	return false
}

type section struct {
	name  string
	lines []string
}

type entry struct {
	section  string
	title    string
	body     []string
	captured []section
}

type record struct {
	word, label, test, message string
	used                       bool
}

type parser struct {
	r        Report
	rootdir  string
	section  string
	entries  []*entry
	cur      *entry
	records  []*record
	warnings warningParser
	found    bool
}

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
	text := strings.ReplaceAll(string(input), "\r\n", "\n")
	for _, raw := range strings.Split(text, "\n") {
		p.line(core.Line(raw))
	}
	p.closeEntry()
	if !p.found {
		return Report{}, false
	}
	p.finish(o)
	return p.r, true
}

func (p *parser) relative(l string) string {
	if p.rootdir == "" {
		return l
	}
	return strings.ReplaceAll(l, p.rootdir+"/", "")
}

func (p *parser) inCaptured() bool {
	return p.cur != nil && len(p.cur.captured) > 0
}

func (p *parser) line(l string) {
	framed := strings.HasPrefix(l, "=")
	if m := statsLine.FindStringSubmatch(l); m != nil && (framed || !p.inCaptured()) && (p.found || framed || knownStats(m[1])) {
		p.found = true
		p.closeEntry()
		p.stats(m[1], m[2])
		p.section = ""
		return
	}
	if sessionStart.MatchString(l) {
		p.found = true
		p.closeEntry()
		p.section = "header"
		return
	}
	if m := sectionLine.FindStringSubmatch(l); m != nil && (!p.inCaptured() || knownSection.MatchString(l)) {
		p.closeEntry()
		p.section = m[1]
		if knownSection.MatchString(l) {
			p.found = true
		}
		return
	}
	if m := bangLine.FindStringSubmatch(l); m != nil && !p.inCaptured() {
		p.closeEntry()
		msg := m[1]
		if rest, ok := strings.CutPrefix(msg, "Interrupted: "); ok {
			p.r.Interrupted = rest
		} else {
			p.r.Stopped = msg
		}
		return
	}
	switch p.section {
	case "header":
		p.headerLine(l)
	case "":
		p.note(l)
	case "FAILURES", "ERRORS":
		p.entryLine(l)
	case "short test summary info":
		if m := summaryEntry.FindStringSubmatch(l); m != nil && (m[2] == "FAILED" || m[2] == "ERROR") {
			test, message := splitSummary(m[4])
			p.records = append(p.records, &record{word: m[2], label: strings.TrimSpace(m[3]), test: test, message: message})
		} else if m != nil {
			p.r.Outcomes = append(p.r.Outcomes, p.relative(l))
		} else if strings.TrimSpace(l) != "" {
			p.note(l)
		}
	default:
		if strings.HasPrefix(p.section, "warnings summary") {
			p.warnings.line(p.relative(l))
			return
		}
		if strings.TrimSpace(l) == "" {
			return
		}
		if _, seen := p.r.Sections[p.section]; !seen {
			p.r.sectionOrder = append(p.r.sectionOrder, p.section)
		}
		p.r.Sections[p.section]++
		p.r.sectionLines[p.section] = append(p.r.sectionLines[p.section], p.relative(l))
	}
}

func splitSummary(rest string) (test, message string) {
	depth := 0
	for i := 0; i+3 <= len(rest); i++ {
		switch rest[i] {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		case ' ':
			if depth == 0 && rest[i:i+3] == " - " {
				return rest[:i], rest[i+3:]
			}
		}
	}
	return rest, ""
}

func (p *parser) headerLine(l string) {
	switch {
	case strings.HasPrefix(l, "platform "):
		if m := versionField.FindStringSubmatch(l); m != nil {
			p.r.Version = m[1]
		}
	case strings.HasPrefix(l, "rootdir: "):
		p.rootdir = strings.TrimSpace(strings.TrimPrefix(l, "rootdir: "))
	default:
		p.note(l)
	}
}

func (p *parser) note(l string) {
	t := strings.TrimSpace(l)
	if t == "" || progressLine.MatchString(t) || verboseLine.MatchString(t) || headerLine.MatchString(t) {
		return
	}
	if strings.HasPrefix(t, "INTERNALERROR>") {
		p.r.InternalError = true
	}
	p.r.Notes = append(p.r.Notes, p.relative(l))
}

func (p *parser) entryLine(l string) {
	if m := entryTitle.FindStringSubmatch(l); m != nil {
		p.closeEntry()
		p.cur = &entry{section: p.section, title: m[1]}
		return
	}
	if p.cur == nil {
		p.cur = &entry{section: p.section}
	}
	if p.cur.title == "" && (crashLine.MatchString(l) || (len(p.cur.body) > 0 && strings.HasPrefix(l, "["))) {
		p.cur.body = append(p.cur.body, p.relative(l))
		p.closeEntry()
		return
	}
	if m := capturedTitle.FindStringSubmatch(l); m != nil {
		name, ok := strings.CutPrefix(m[1], "Captured ")
		if !ok {
			p.closeEntry()
			p.section = ""
			return
		}
		p.cur.captured = append(p.cur.captured, section{name: name})
		return
	}
	if n := len(p.cur.captured); n > 0 {
		p.cur.captured[n-1].lines = append(p.cur.captured[n-1].lines, p.relative(l))
		return
	}
	p.cur.body = append(p.cur.body, p.relative(l))
}

func (p *parser) closeEntry() {
	if p.cur != nil {
		p.entries = append(p.entries, p.cur)
		p.cur = nil
	}
}

func (p *parser) stats(parts, duration string) {
	s := &p.r.Summary
	s.Duration = duration
	s.Line = parts + " in " + duration
	p.r.Complete = true
	if parts == "no tests ran" {
		return
	}
	for _, part := range strings.Split(parts, ", ") {
		m := statPart.FindStringSubmatch(part)
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "failed":
			s.Failed = n
		case "passed":
			s.Passed = n
		case "skipped":
			s.Skipped = n
		case "deselected":
			s.Deselected = n
		case "xfailed":
			s.XFailed = n
		case "xpassed":
			s.XPassed = n
		case "warning", "warnings":
			s.Warnings = n
		case "error", "errors":
			s.Errors = n
		default:
			s.Other[m[2]] = n
		}
	}
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

func signature(f Failure) string {
	if f.Location == "" {
		return ""
	}
	parts := []string{f.Kind, f.When, f.Location}
	for _, l := range f.Lines {
		if strings.HasPrefix(l, ">") || location.MatchString(l) {
			parts = append(parts, l)
		}
	}
	return strings.Join(parts, "\n")
}

func foldFailures(failures []Failure) ([]Failure, int) {
	first := map[string]int{}
	out := failures[:0:0]
	folded := 0
	for _, f := range failures {
		sig := signature(f)
		if i, ok := first[sig]; ok && sig != "" {
			name := f.Test
			if f.Subtest != "" {
				name += " " + f.Subtest
			}
			out[i].Same = append(out[i].Same, name)
			folded++
			continue
		}
		if sig != "" {
			first[sig] = len(out)
		}
		out = append(out, f)
	}
	return out, folded
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
	any := false
	for i, l := range all {
		switch {
		case isE(l), strings.HasPrefix(l, ">"), location.MatchString(l):
			keep[i], any = true, true
		case i > 0 && inFunction.MatchString(all[i-1]) && strings.HasPrefix(l, " "):
			keep[i] = true
		}
	}
	if !any {
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

type warningParser struct {
	locations int
	message   string
	closed    bool
	open      bool
	order     []string
	byMsg     map[string]int
}

func (w *warningParser) flush() {
	if w.open && (w.message != "" || w.locations > 0) {
		if w.byMsg == nil {
			w.byMsg = map[string]int{}
		}
		if _, seen := w.byMsg[w.message]; !seen {
			w.order = append(w.order, w.message)
		}
		w.byMsg[w.message] += w.locations
	}
	w.locations, w.message, w.closed, w.open = 0, "", false, false
}

func (w *warningParser) line(l string) {
	t := strings.TrimSpace(l)
	switch {
	case strings.HasPrefix(t, "-- Docs: "):
		w.flush()
	case t == "":
		w.closed = w.message != ""
	case !strings.HasPrefix(l, " ") && !isWarningLocation(t):
		if w.message != "" && w.locations == 0 && !w.closed {
			w.message += " " + t
			return
		}
		w.flush()
		w.open, w.message = true, t
	case !strings.HasPrefix(l, " "):
		if w.message != "" {
			w.flush()
		}
		w.open = true
		if m := locationCount.FindStringSubmatch(t); m != nil {
			n, _ := strconv.Atoi(m[1])
			w.locations += n
		} else {
			w.locations++
		}
	case strings.HasPrefix(l, "    "):
		w.closed = true
	case w.message == "":
		w.open, w.message = true, t
	case !w.closed:
		w.message += " " + t
	}
}

func isWarningLocation(t string) bool {
	return !strings.Contains(t, " ") || locationCount.MatchString(t)
}

func (w *warningParser) result() []Warning {
	w.flush()
	out := make([]Warning, 0, len(w.order))
	for _, msg := range w.order {
		out = append(out, Warning{Message: msg, Count: w.byMsg[msg]})
	}
	return out
}

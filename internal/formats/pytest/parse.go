package pytest

import (
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

var (
	sessionStart  = regexp.MustCompile(`^=+ test session starts =+$`)
	sectionLine   = regexp.MustCompile(`^=+ (.+?) =+$`)
	knownSection  = regexp.MustCompile(`^=+ (FAILURES|ERRORS|short test summary info|warnings summary(?: \(final\))?) =+$`)
	reportSection = regexp.MustCompile(`^=+ (FAILURES|ERRORS|XFAILURES|PASSES|XPASSES|short test summary info|warnings summary(?: \(final\))?|tests coverage) =+$`)
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
	kind                 failureKind
	label, test, message string
	used                 bool
}

type parser struct {
	r        report
	rootdir  string
	section  string
	entries  []*entry
	cur      *entry
	records  []*record
	index    recordIndex
	warnings warningParser
	found    bool
}

func (p *parser) relative(l string) string {
	return core.TrimRoot(l, p.rootdir)
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
	if m := sectionLine.FindStringSubmatch(l); m != nil && (!p.inCaptured() || reportSection.MatchString(l)) {
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
			kind := kindFailed
			if m[2] == "ERROR" {
				kind = kindError
			}
			p.records = append(p.records, &record{kind: kind, label: strings.TrimSpace(m[3]), test: test, message: message})
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
		i := slices.IndexFunc(p.r.OtherSectionLines, func(s sectionLines) bool { return s.Name == p.section })
		if i < 0 {
			i = len(p.r.OtherSectionLines)
			p.r.OtherSectionLines = append(p.r.OtherSectionLines, sectionLines{Name: p.section})
		}
		p.r.OtherSections[p.section]++
		p.r.OtherSectionLines[i].Lines = append(p.r.OtherSectionLines[i].Lines, p.relative(l))
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
			p.r.ToolVersion = m[1]
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
	p.r.SummaryFound = true
	if parts == "no tests ran" {
		return
	}
	for _, part := range strings.Split(parts, ", ") {
		m := statPart.FindStringSubmatch(part)
		if m == nil {
			continue
		}
		n := parseCount(m[1])
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

func parseCount(digits string) int {
	n, err := strconv.Atoi(digits)
	if err != nil || n > math.MaxInt32 {
		return math.MaxInt32
	}
	return n
}

func addCounts(a, b int) int {
	return min(a+b, math.MaxInt32)
}

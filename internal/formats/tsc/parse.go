package tsc

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

var ErrNotTSC = errors.New("no tsc diagnostic, summary or build status line in the input")

const clock = `\d{1,2}:\d\d:\d\d(?:[ \x{202f}]?[AaPp]\.?[Mm]\.?)?`

var (
	plainHeader  = regexp.MustCompile(`^(.+?)\((\d+),(\d+)\): (error|warning|message|suggestion) TS(\d+): (.*)$`)
	prettyHeader = regexp.MustCompile(`^(.+?):(\d+):(\d+) - (error|warning|message|suggestion) TS(\d+): (.*)$`)
	globalHeader = regexp.MustCompile(`^(error|warning|message|suggestion) TS(\d+): (.*)$`)
	status       = regexp.MustCompile(`^(?:\[` + clock + `\]|` + clock + ` -) (.*)$`)
	projectItem  = regexp.MustCompile(`^    \* \S`)
	summaryLine  = regexp.MustCompile(`^Found (\d+) errors?(?:\.|\. Watching for file changes\.| in (?:the same file, starting at: )?(.+):\d+| in (\d+) files\.)$`)
	tableRow     = regexp.MustCompile(`^\s*\d+  \S.*:\d+$`)
	relatedLoc   = regexp.MustCompile(`^  (\S.*?):(\d+):(\d+)(?: - (.*))?$`)
	frameLine    = regexp.MustCompile(`^\s*(?:\d+|\.\.\.)(?: |$)|^[\s~]+$`)
)

const tableHeader = "Errors  Files"

type Log struct {
	Diagnostics   []tfdiag.Diagnostic
	Summary       string
	SummaryErrors int
	SummaryFiles  int
	Other         []string
}

type state int

const (
	idle state = iota
	chain
	tail
	table
)

type related struct {
	loc string
	msg []string
}

type parser struct {
	log     Log
	state   state
	pretty  bool
	chain   []string
	related []related
	found   bool
}

func parseHeader(line string) (tfdiag.Diagnostic, bool, bool) {
	if m := plainHeader.FindStringSubmatch(line); m != nil {
		return diagnostic(m[4], m[5], m[6], m[1], m[2], m[3]), true, false
	}
	if m := prettyHeader.FindStringSubmatch(line); m != nil {
		return diagnostic(m[4], m[5], m[6], m[1], m[2], m[3]), true, true
	}
	if m := globalHeader.FindStringSubmatch(line); m != nil {
		return diagnostic(m[1], m[2], m[3], "", "", ""), true, true
	}
	return tfdiag.Diagnostic{}, false, false
}

func diagnostic(category, code, msg, file, line, col string) tfdiag.Diagnostic {
	d := tfdiag.Diagnostic{Severity: category, Summary: core.Clean(msg), Address: "TS" + code, File: core.Clean(file)}
	d.Line, _ = strconv.Atoi(line)
	d.Column, _ = strconv.Atoi(col)
	return d
}

func detect(head []byte) bool {
	build := false
	for _, raw := range strings.Split(string(head), "\n") {
		line := core.Line(raw)
		if m := status.FindStringSubmatch(line); m != nil {
			build = build || strings.HasPrefix(m[1], "Projects in this build:") || strings.HasPrefix(m[1], "Starting compilation in watch mode")
			continue
		}
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "> ") || projectItem.MatchString(line) {
			continue
		}
		_, ok, _ := parseHeader(line)
		return ok
	}
	return build
}

func Parse(input []byte) (*Log, error) {
	p := &parser{}
	for _, raw := range strings.Split(string(input), "\n") {
		p.line(core.Line(raw))
	}
	p.flush()
	if !p.found {
		return nil, ErrNotTSC
	}
	return &p.log, nil
}

func (p *parser) line(line string) {
	if d, ok, pretty := parseHeader(line); ok {
		p.flush()
		p.log.Diagnostics = append(p.log.Diagnostics, d)
		p.state, p.pretty, p.found = chain, pretty, true
		return
	}
	if strings.TrimSpace(line) == "" {
		if p.state == chain {
			p.state = idle
			if p.pretty {
				p.state = tail
			}
		}
		return
	}
	switch p.state {
	case chain:
		if strings.HasPrefix(line, "  ") {
			p.chain = append(p.chain, strings.TrimSpace(line))
			return
		}
	case tail:
		if strings.HasPrefix(line, " ") || frameLine.MatchString(line) {
			p.tail(line)
			return
		}
	case table:
		if line == tableHeader || tableRow.MatchString(line) {
			return
		}
	case idle:
	}
	p.flush()
	p.state = idle
	p.other(line)
}

func (p *parser) tail(line string) {
	if m := relatedLoc.FindStringSubmatch(line); m != nil {
		r := related{loc: core.Clean(m[1]) + ":" + m[2] + ":" + m[3]}
		if m[4] != "" {
			r.msg = append(r.msg, m[4])
		}
		p.related = append(p.related, r)
		return
	}
	if frameLine.MatchString(line) {
		return
	}
	if len(p.related) == 0 {
		p.related = append(p.related, related{})
	}
	last := &p.related[len(p.related)-1]
	last.msg = append(last.msg, strings.TrimSpace(line))
}

func (p *parser) flush() {
	if n := len(p.log.Diagnostics); n > 0 {
		d := &p.log.Diagnostics[n-1]
		if len(p.chain) > 0 {
			d.Detail = strings.Join(p.chain, "\n")
		}
		for _, r := range p.related {
			v := core.Clean(strings.Join(r.msg, " "))
			if r.loc != "" {
				v = strings.TrimSuffix(r.loc+": "+v, ": ")
			}
			d.Values = append(d.Values, v)
		}
	}
	p.chain, p.related = nil, nil
}

func (p *parser) other(line string) {
	msg, isStatus := line, false
	if m := status.FindStringSubmatch(line); m != nil {
		msg, isStatus = m[1], true
		p.found = true
	}
	if m := summaryLine.FindStringSubmatch(msg); m != nil {
		p.log.Summary = core.Clean(msg)
		p.log.SummaryErrors, _ = strconv.Atoi(m[1])
		p.log.SummaryFiles = 0
		if m[2] != "" {
			p.log.SummaryFiles = 1
		} else if m[3] != "" {
			p.log.SummaryFiles, _ = strconv.Atoi(m[3])
		}
		p.state, p.found = table, true
		return
	}
	clean := core.Clean(line)
	switch {
	case isStatus || projectItem.MatchString(line):
	case core.ErrorLine(clean):
		p.log.Diagnostics = append(p.log.Diagnostics, tfdiag.Diagnostic{Severity: "error", Summary: clean})
		return
	case core.WarningLine(clean):
		p.log.Diagnostics = append(p.log.Diagnostics, tfdiag.Diagnostic{Severity: "warning", Summary: clean})
		return
	}
	p.log.Other = append(p.log.Other, strings.TrimRight(line, " "))
}

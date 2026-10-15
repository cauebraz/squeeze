package precommit

import (
	"bytes"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

var ErrNoRun = errors.New("no pre-commit hook status line or message in the input")

type Status string

const (
	Passed     Status = "passed"
	Failed     Status = "failed"
	Skipped    Status = "skipped"
	Unfinished Status = "unfinished"
)

const (
	minWidth = 79
	endLen   = 6
	noFiles  = "(no files to check)"
)

var (
	statusLine   = regexp.MustCompile(`^(\S.*?)\.{3,}(\(no files to check\))?(Passed|Failed|Skipped)$`)
	fatalLine    = regexp.MustCompile(`^(An error has occurred|An unexpected error has occurred|Interrupted \(\^C\)): `)
	startedLine  = regexp.MustCompile(`^(\S.*?)\.{3,}((?:An error has occurred|An unexpected error has occurred|Interrupted \(\^C\)): .*)$`)
	hookID       = regexp.MustCompile(`^- hook id: (.*)$`)
	duration     = regexp.MustCompile(`^- duration: (\d+(?:\.\d+)?)s$`)
	exitCode     = regexp.MustCompile(`^- exit code: (-?\d+)$`)
	diffFile     = regexp.MustCompile(`^diff --git a/(.*) b/`)
	noHook       = regexp.MustCompile("^No hook with id `.*` in stage `.*`$")
	modifiedLine = "- files were modified by this hook"
	madeChanges  = "pre-commit hook(s) made changes."
	allChanges   = "All changes made by hooks:"
)

type Hook struct {
	Name     string  `json:"name"`
	ID       string  `json:"id"`
	Status   Status  `json:"status"`
	NoFiles  bool    `json:"no_files"`
	ExitCode int     `json:"exit_code"`
	Modified bool    `json:"files_modified"`
	Duration float64 `json:"duration_seconds"`
	lines    []string
}

type Run struct {
	Hooks        []Hook
	Errors       []string
	Warnings     []string
	Other        []string
	ChangedFiles []string
	Diff         []string
}

func width(s string) int {
	n := 0
	for _, r := range s {
		n++
		if wide(r) {
			n++
		}
	}
	return n
}

func wide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115f, r >= 0x2e80 && r <= 0xa4cf, r >= 0xac00 && r <= 0xd7a3:
		return true
	case r >= 0xf900 && r <= 0xfaff, r >= 0xfe30 && r <= 0xfe4f, r >= 0xff00 && r <= 0xff60, r >= 0xffe0 && r <= 0xffe6:
		return true
	case r >= 0x1f300 && r <= 0x1f64f, r >= 0x1f900 && r <= 0x1f9ff, r >= 0x20000 && r <= 0x3fffd:
		return true
	}
	return false
}

func clean(raw []byte) (string, bool) {
	raw = bytes.TrimRight(raw, "\r")
	if bytes.IndexByte(raw, '\t') >= 0 {
		return "", false
	}
	return core.Line(string(raw)), true
}

func detect(head []byte) bool {
	for _, raw := range bytes.Split(head, []byte("\n")) {
		l, ok := clean(raw)
		if !ok {
			continue
		}
		if statusLine.MatchString(l) && width(l) >= minWidth {
			return true
		}
		if m := startedLine.FindStringSubmatchIndex(l); m != nil && width(l[:m[4]]) >= minWidth-endLen {
			return true
		}
	}
	return false
}

type state int

const (
	top state = iota
	meta
	output
	trailer
	diff
)

type parser struct {
	run   Run
	cols  int
	state state
	found bool
}

func (p *parser) hook() *Hook {
	return &p.run.Hooks[len(p.run.Hooks)-1]
}

func (p *parser) status(l string) bool {
	m := statusLine.FindStringSubmatch(l)
	if m == nil {
		return false
	}
	w := width(l)
	if (p.cols == 0 && w < minWidth) || (p.cols != 0 && w != p.cols) {
		return false
	}
	p.cols = w
	h := Hook{Name: core.Clean(m[1]), Status: Status(strings.ToLower(m[3])), NoFiles: m[2] == noFiles}
	p.run.Hooks = append(p.run.Hooks, h)
	p.state = meta
	return true
}

func (p *parser) started(l string) bool {
	m := startedLine.FindStringSubmatchIndex(l)
	if m == nil {
		return false
	}
	w := width(l[:m[4]])
	if (p.cols == 0 && w < minWidth-endLen) || (p.cols != 0 && w != p.cols-endLen) {
		return false
	}
	p.run.Hooks = append(p.run.Hooks, Hook{Name: core.Clean(l[m[2]:m[3]]), Status: Unfinished})
	p.run.Errors = append(p.run.Errors, core.Clean(l[m[4]:m[5]]))
	p.state = top
	return true
}

func (p *parser) meta(l string) bool {
	h := p.hook()
	if m := hookID.FindStringSubmatch(l); m != nil {
		h.ID = core.Clean(m[1])
	} else if m := duration.FindStringSubmatch(l); m != nil {
		h.Duration, _ = strconv.ParseFloat(m[1], 64)
	} else if m := exitCode.FindStringSubmatch(l); m != nil {
		h.ExitCode, _ = strconv.Atoi(m[1])
	} else if l == modifiedLine {
		h.Modified = true
	} else {
		return false
	}
	return true
}

func (p *parser) top(l string) {
	switch {
	case strings.TrimSpace(l) == "":
	case strings.HasPrefix(l, "[INFO] "):
		p.found = true
	case strings.HasPrefix(l, "[WARNING] "):
		p.found = true
		p.run.Warnings = append(p.run.Warnings, core.Clean(strings.TrimPrefix(l, "[WARNING] ")))
	case strings.HasPrefix(l, "[ERROR] "):
		p.found = true
		p.run.Errors = append(p.run.Errors, core.Clean(strings.TrimPrefix(l, "[ERROR] ")))
	case noHook.MatchString(l):
		p.found = true
		p.run.Errors = append(p.run.Errors, core.Clean(l))
	default:
		p.run.Other = append(p.run.Other, l)
	}
}

func (p *parser) line(l string) {
	if p.status(l) || p.started(l) {
		return
	}
	if fatalLine.MatchString(l) {
		p.found = true
		p.run.Errors = append(p.run.Errors, core.Clean(l))
		p.state = top
		return
	}
	if l == madeChanges && p.state != diff {
		p.state = trailer
		return
	}
	if l == allChanges && p.state != diff {
		p.state = diff
		return
	}
	switch p.state {
	case meta:
		if p.meta(l) {
			return
		}
		if strings.TrimSpace(l) == "" {
			p.state = output
			return
		}
		p.state = top
		p.top(l)
	case output:
		h := p.hook()
		h.lines = append(h.lines, l)
	case trailer:
	case diff:
		if m := diffFile.FindStringSubmatch(l); m != nil {
			p.run.ChangedFiles = append(p.run.ChangedFiles, core.Clean(m[1]))
		}
		p.run.Diff = append(p.run.Diff, l)
	case top:
		p.top(l)
	}
}

func Parse(input []byte) (Run, error) {
	p := parser{run: Run{Hooks: []Hook{}, Errors: []string{}, Warnings: []string{}, Other: []string{}, ChangedFiles: []string{}, Diff: []string{}}}
	for _, raw := range bytes.Split(input, []byte("\n")) {
		raw = bytes.TrimRight(raw, "\r")
		p.line(core.Line(string(raw)))
	}
	if len(p.run.Hooks) == 0 && !p.found {
		return Run{}, ErrNoRun
	}
	for i := range p.run.Hooks {
		p.run.Hooks[i].lines = trimBlank(p.run.Hooks[i].lines)
	}
	p.run.Other = trimBlank(p.run.Other)
	return p.run, nil
}

func trimBlank(lines []string) []string {
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

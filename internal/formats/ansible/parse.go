package ansible

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

var ErrNoRun = errors.New("no ansible-playbook PLAY banner, PLAY RECAP or json callback document in the input")

type Status string

const (
	StatusOK          Status = "ok"
	StatusChanged     Status = "changed"
	StatusSkipped     Status = "skipped"
	StatusFailed      Status = "failed"
	StatusUnreachable Status = "unreachable"
	StatusIgnored     Status = "ignored"
	StatusRescued     Status = "rescued"
	StatusRecovered   Status = "recovered"
)

type Detail struct {
	Msg          string `json:"msg,omitempty"`
	RC           string `json:"rc,omitempty"`
	Stderr       string `json:"stderr,omitempty"`
	Stdout       string `json:"stdout,omitempty"`
	ModuleStderr string `json:"module_stderr,omitempty"`
	ModuleStdout string `json:"module_stdout,omitempty"`
	Censored     string `json:"censored,omitempty"`
}

type Event struct {
	Play    string
	Task    string
	Path    string
	Host    string
	Item    string
	Status  Status
	Ignored bool
	Seq     int
	Detail  Detail
}

type HostRecap struct {
	Host        string `json:"host"`
	OK          int    `json:"ok"`
	Changed     int    `json:"changed"`
	Unreachable int    `json:"unreachable"`
	Failed      int    `json:"failed"`
	Skipped     int    `json:"skipped"`
	Rescued     int    `json:"rescued"`
	Ignored     int    `json:"ignored"`
}

type Run struct {
	Input    string
	Version  string
	Plays    int
	Events   []Event
	Recap    []HostRecap
	HasRecap bool
	Errors   []string
	Warnings []string
}

var (
	playBanner  = regexp.MustCompile(`^PLAY(?: \[(.*)\])? \*{3,}$`)
	taskBanner  = regexp.MustCompile(`^(?:TASK|RUNNING HANDLER) \[(.*)\] \*{3,}$`)
	recapBanner = regexp.MustCompile(`^PLAY RECAP \*{3,}$`)
	anyBanner   = regexp.MustCompile(`^[A-Z][A-Z :]*(?: \[.*\])? \*{3,}$`)
	versionLine = regexp.MustCompile(`^ansible-playbook (?:\[core )?([0-9][^\] ]*)\]?$`)
	taskPath    = regexp.MustCompile(`^task path: (.+)$`)
	origin      = regexp.MustCompile(`^Origin: (.+)$`)
	fatalLine   = regexp.MustCompile(`^fatal: \[(.+?)\]: (FAILED|UNREACHABLE)! => (.*)$`)
	itemLine    = regexp.MustCompile(`^failed: \[(.+?)\] \(item=(.*?)\) => (.*)$`)
	resultLine  = regexp.MustCompile(`^(ok|changed|skipping): \[(.+?)\](?: => \(item=(.*?)\))?(?: => (.*?))?\s*$`)
	recapLine   = regexp.MustCompile(`^(\S+)\s+: ok=(\d+)\s+changed=(\d+)\s+unreachable=(\d+)\s+failed=(\d+)\s+skipped=(\d+)(?:\s+rescued=(\d+))?(?:\s+ignored=(\d+))?\s*$`)
)

func detectText(head []byte) bool {
	play, other := false, false
	for _, raw := range bytes.Split(head, []byte("\n")) {
		l := core.Line(string(raw))
		switch {
		case playBanner.MatchString(l):
			play = true
		case taskBanner.MatchString(l), recapBanner.MatchString(l), l == "skipping: no hosts matched":
			other = true
		}
		if play && other {
			return true
		}
	}
	return false
}

type textParser struct {
	run       Run
	play      string
	task      string
	path      string
	seq       int
	recap     bool
	pending   []string
	pendingAt string
	lines     []string
	i         int
	found     bool
}

func Parse(input []byte) (Run, error) {
	if detectJSON(core.Head(input)) || detectJSON(input) {
		return parseJSON(input)
	}
	p := &textParser{lines: strings.Split(string(input), "\n")}
	p.parse()
	if !p.found {
		return Run{}, ErrNoRun
	}
	p.run.Input = "text"
	return p.run, nil
}

func (p *textParser) flush() {
	p.run.Errors = append(p.run.Errors, p.pending...)
	p.pending, p.pendingAt = nil, ""
}

func (p *textParser) parse() {
	for ; p.i < len(p.lines); p.i++ {
		p.line(core.Line(p.lines[p.i]))
	}
	p.flush()
}

func (p *textParser) line(l string) {
	trimmed := strings.TrimSpace(l)
	if m := versionLine.FindStringSubmatch(l); m != nil && p.run.Version == "" {
		p.run.Version = core.Clean(m[1])
		return
	}
	if p.marker(trimmed) {
		return
	}
	if anyBanner.MatchString(l) {
		p.banner(l)
		return
	}
	if p.recap {
		if m := recapLine.FindStringSubmatch(l); m != nil {
			p.run.Recap = append(p.run.Recap, recapOf(m))
		}
		return
	}
	if m := taskPath.FindStringSubmatch(l); m != nil {
		p.path = core.Clean(m[1])
		return
	}
	if m := origin.FindStringSubmatch(l); m != nil {
		p.pendingAt = core.Clean(m[1])
		return
	}
	if m := fatalLine.FindStringSubmatch(l); m != nil {
		status := StatusFailed
		if m[2] == "UNREACHABLE" {
			status = StatusUnreachable
		}
		p.result(m[1], "", status, m[3])
		return
	}
	if m := itemLine.FindStringSubmatch(l); m != nil {
		p.result(m[1], m[2], StatusFailed, m[3])
		return
	}
	if m := resultLine.FindStringSubmatch(l); m != nil {
		status := StatusOK
		switch m[1] {
		case "changed":
			status = StatusChanged
		case "skipping":
			status = StatusSkipped
		}
		p.result(m[2], m[3], status, m[4])
		return
	}
	if trimmed == "...ignoring" {
		p.ignore()
	}
}

func (p *textParser) marker(l string) bool {
	for _, prefix := range []string{"[ERROR]: ", "ERROR! "} {
		if rest, ok := strings.CutPrefix(l, prefix); ok {
			p.flush()
			p.pending = []string{core.Clean(rest)}
			return true
		}
	}
	for _, prefix := range []string{"[WARNING]: ", "[DEPRECATION WARNING]: "} {
		if rest, ok := strings.CutPrefix(l, prefix); ok {
			p.run.Warnings = append(p.run.Warnings, core.Clean(rest))
			return true
		}
	}
	return false
}

func (p *textParser) banner(l string) {
	p.flush()
	p.recap = false
	switch {
	case recapBanner.MatchString(l):
		p.recap, p.found, p.run.HasRecap = true, true, true
	case playBanner.MatchString(l):
		p.play = core.Clean(playBanner.FindStringSubmatch(l)[1])
		p.task, p.path = "", ""
		p.run.Plays++
		p.found = true
	case taskBanner.MatchString(l):
		name := taskBanner.FindStringSubmatch(l)[1]
		name = strings.TrimSuffix(name, "] [CHECK MODE")
		p.task, p.path = core.Clean(name), ""
		p.seq++
	}
}

func (p *textParser) blob(rest string) map[string]json.RawMessage {
	rest = strings.TrimSpace(rest)
	if rest == "{" {
		var b strings.Builder
		b.WriteString("{")
		for p.i+1 < len(p.lines) {
			next := strings.TrimRight(p.lines[p.i+1], "\r")
			if anyBanner.MatchString(core.Line(next)) {
				break
			}
			p.i++
			b.WriteString(next)
			if next == "}" {
				break
			}
		}
		rest = b.String()
	}
	var m map[string]json.RawMessage
	if json.Unmarshal([]byte(rest), &m) != nil {
		return nil
	}
	return m
}

func (p *textParser) result(host, item string, status Status, rest string) {
	m := p.blob(rest)
	if status == StatusOK || status == StatusChanged || status == StatusSkipped {
		p.add(host, item, status, Detail{})
		return
	}
	d := detailOf(m)
	if d.Msg == "" && len(p.pending) > 0 {
		d.Msg = p.pending[0]
	}
	path := p.path
	if path == "" {
		path = p.pendingAt
	}
	p.pending, p.pendingAt = nil, ""
	p.addAt(host, item, status, d, path)
}

func (p *textParser) add(host, item string, status Status, d Detail) {
	p.addAt(host, item, status, d, p.path)
}

func (p *textParser) addAt(host, item string, status Status, d Detail, path string) {
	p.found = true
	p.run.Events = append(p.run.Events, Event{
		Play: p.play, Task: p.task, Path: path, Host: core.Clean(host), Item: core.Clean(item),
		Status: status, Seq: p.seq, Detail: d,
	})
}

func (p *textParser) ignore() {
	events := p.run.Events
	host := ""
	for i := len(events) - 1; i >= 0 && events[i].Seq == p.seq; i-- {
		e := &events[i]
		if e.Status != StatusFailed && e.Status != StatusUnreachable || (host != "" && e.Host != host) {
			continue
		}
		if e.Ignored {
			return
		}
		host = e.Host
		e.Ignored = true
	}
}

func recapOf(m []string) HostRecap {
	n := func(s string) int {
		v, _ := strconv.Atoi(s)
		return v
	}
	return HostRecap{
		Host: core.Clean(m[1]), OK: n(m[2]), Changed: n(m[3]), Unreachable: n(m[4]),
		Failed: n(m[5]), Skipped: n(m[6]), Rescued: n(m[7]), Ignored: n(m[8]),
	}
}

func detailOf(m map[string]json.RawMessage) Detail {
	return Detail{
		Msg:          rawText(m["msg"]),
		RC:           rawText(m["rc"]),
		Stderr:       rawText(m["stderr"]),
		Stdout:       rawText(m["stdout"]),
		ModuleStderr: rawText(m["module_stderr"]),
		ModuleStdout: rawText(m["module_stdout"]),
		Censored:     rawText(m["censored"]),
	}
}

func rawText(r json.RawMessage) string {
	r = bytes.TrimSpace(r)
	if len(r) == 0 || string(r) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(r, &s) == nil {
		return s
	}
	var b bytes.Buffer
	if json.Compact(&b, r) != nil {
		return ""
	}
	return b.String()
}

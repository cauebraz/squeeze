package cargo

import (
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

var ErrNotCargo = errors.New("input holds no cargo message, test result or build status")

type Diagnostic struct {
	tfdiag.Diagnostic
	ID      string `json:"id"`
	Fixable bool   `json:"fixable,omitempty"`
}

type Target struct {
	Name        string   `json:"name"`
	Result      string   `json:"result"`
	Passed      int      `json:"passed"`
	Failed      int      `json:"failed"`
	Ignored     int      `json:"ignored"`
	Measured    int      `json:"measured"`
	FilteredOut int      `json:"filtered_out"`
	Rerun       string   `json:"rerun,omitempty"`
	Cause       []string `json:"cause,omitempty"`
}

type Failure struct {
	Target  string   `json:"target"`
	Test    string   `json:"test"`
	File    string   `json:"file,omitempty"`
	Line    int      `json:"line,omitempty"`
	Column  int      `json:"column,omitempty"`
	Message []string `json:"message"`
	Output  []string `json:"output"`
	index   int
}

type Run struct {
	Diagnostics   []Diagnostic
	BuildFinished *bool
	Finished      bool
	FailedUnits   []string
	Targets       []Target
	Failures      []Failure
}

var (
	runningRe         = regexp.MustCompile(`^running (\d+) tests?$`)
	resultRe          = regexp.MustCompile(`^test result: (ok|FAILED)\. (\d+) passed; (\d+) failed; (\d+) ignored; (\d+) measured; (\d+) filtered out`)
	testLineRe        = regexp.MustCompile(`^test (.+?) \.\.\. (ok|FAILED|ignored)(, .*)?$`)
	blockRe           = regexp.MustCompile(`^---- (.+) (stdout|stderr) ----$`)
	couldNotCompileRe = regexp.MustCompile("^error: could not compile `([^`]+)`(?: \\(([^)]*)\\))?")
	rerunRe           = regexp.MustCompile("^error: (?:test|doctest) failed, to rerun pass `([^`]*)`")
	targetsFailedRe   = regexp.MustCompile(`^error: \d+ targets? failed:$`)
	generatedRe       = regexp.MustCompile("^warning: `[^`]+` \\(.*\\) generated \\d+ warnings?")
	headerRe          = regexp.MustCompile(`^(error|warning)(?:\[([^\]]+)\])?: (.*)$`)
	unitRe            = regexp.MustCompile(`^ +Running (.+) \((.+)\)$`)
	docRe             = regexp.MustCompile(`^ +Doc-tests (\S+)$`)
	hashRe            = regexp.MustCompile(`-[0-9a-f]{16}$`)
	panicRe           = regexp.MustCompile(`^thread '.*'(?: \(\d+\))? panicked at (.+):(\d+):(\d+):$`)
	oldPanicRe        = regexp.MustCompile(`^thread '.*' panicked at '(.*)', (.+):(\d+):(\d+)$`)
)

type parser struct {
	lines   []string
	run     Run
	pending string
	seen    bool
}

func Parse(input []byte) (*Run, error) {
	p := &parser{lines: strings.Split(strings.ReplaceAll(string(input), "\r\n", "\n"), "\n")}
	for i := 0; i < len(p.lines); {
		i = p.line(i)
	}
	if !p.seen {
		return nil, ErrNotCargo
	}
	return &p.run, nil
}

func (p *parser) line(i int) int {
	line := p.lines[i]
	if isMessage(line) && p.message(line) {
		p.seen = true
		return i + 1
	}
	if m := runningRe.FindStringSubmatch(line); m != nil {
		p.seen = true
		return p.target(i + 1)
	}
	if m := unitRe.FindStringSubmatch(line); m != nil {
		p.pending = unitName(m[1], m[2])
		return i + 1
	}
	if m := docRe.FindStringSubmatch(line); m != nil {
		p.pending = "doc-tests " + m[1]
		return i + 1
	}
	if finishedRe.MatchString(line) {
		p.seen, p.run.Finished = true, true
		return i + 1
	}
	if m := couldNotCompileRe.FindStringSubmatch(line); m != nil {
		p.seen = true
		unit := m[1]
		if m[2] != "" {
			unit += " (" + m[2] + ")"
		}
		p.run.FailedUnits = append(p.run.FailedUnits, core.Clean(unit))
		return i + 1
	}
	if m := rerunRe.FindStringSubmatch(line); m != nil && len(p.run.Targets) > 0 {
		t := &p.run.Targets[len(p.run.Targets)-1]
		t.Rerun = core.Clean(m[1])
		cause, next := p.cause(i + 1)
		t.Cause = cause
		return next
	}
	if targetsFailedRe.MatchString(line) {
		i++
		for i < len(p.lines) && strings.HasPrefix(p.lines[i], "    ") {
			i++
		}
		return i
	}
	if generatedRe.MatchString(line) || strings.HasPrefix(line, "warning: build failed, waiting for other jobs to finish") {
		return i + 1
	}
	if headerRe.MatchString(line) {
		p.seen = true
		return p.human(i)
	}
	return i + 1
}

func unitName(desc, binary string) string {
	if !strings.HasPrefix(desc, "unittests ") {
		return core.Clean(desc)
	}
	base := binary[strings.LastIndexAny(binary, `/\`)+1:]
	base = strings.TrimSuffix(base, ".exe")
	return core.Clean(desc + " (" + hashRe.ReplaceAllString(base, "") + ")")
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func (p *parser) cause(i int) ([]string, int) {
	j := i
	for j < len(p.lines) && strings.TrimSpace(p.lines[j]) == "" {
		j++
	}
	if j >= len(p.lines) || p.lines[j] != "Caused by:" {
		return nil, i
	}
	var out []string
	for j++; j < len(p.lines); j++ {
		l := p.lines[j]
		if l != "" && l[0] != ' ' && l[0] != '\t' {
			break
		}
		if c := core.Clean(l); c != "" {
			out = append(out, c)
		}
	}
	return out, j
}

func (p *parser) message(line string) bool {
	var m struct {
		Reason  string `json:"reason"`
		Success *bool  `json:"success"`
		Message *Raw   `json:"message"`
	}
	if json.Unmarshal([]byte(line), &m) != nil {
		return false
	}
	switch m.Reason {
	case "build-finished":
		p.run.BuildFinished = m.Success
	case "compiler-message":
		if m.Message != nil {
			if d, ok := convert(*m.Message); ok {
				p.run.Diagnostics = append(p.run.Diagnostics, d)
			}
		}
	}
	return true
}

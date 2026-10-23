package gotest

import (
	"bytes"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

var (
	plainFrame   = regexp.MustCompile(`^=== (RUN|PAUSE|CONT|NAME)\s+(\S+)$`)
	plainResult  = regexp.MustCompile(`^\s*--- (PASS|FAIL|SKIP): (\S+) \(([0-9.]+)s\)$`)
	plainSummary = regexp.MustCompile(`^(ok  |FAIL|\?   )\t(\S+)(?:[\t ](.*))?$`)
	plainBuild   = regexp.MustCompile(`^# (\S+(?: \[\S+\])?)$`)
	plainElapsed = regexp.MustCompile(`^([0-9.]+)s`)
)

var errNotGoTest = errors.New("no go test -json event and no go test text output in the input")

func detectPlain(head []byte) bool {
	for _, l := range bytes.Split(head, []byte("\n")) {
		l = bytes.TrimRight(l, "\r")
		if plainSummary.Match(l) || plainResult.Match(l) || bytes.HasPrefix(l, []byte("=== RUN   ")) {
			return true
		}
	}
	return false
}

type plainLine struct {
	e   rawEvent
	raw string
}

type plainParser struct {
	events  []rawEvent
	pending []plainLine
	builds  map[string]string
	build   string
	current string
	found   bool
}

func parsePlain(input []byte) ([]rawEvent, error) {
	p := plainParser{builds: map[string]string{}}
	for raw := range core.SplitLines(string(input)) {
		p.line(raw)
	}
	if !p.found {
		return nil, errNotGoTest
	}
	for _, l := range p.pending {
		if t := strings.TrimSpace(l.raw); t != "" && t != "PASS" && t != "FAIL" {
			p.events = append(p.events, rawEvent{Text: l.raw})
		}
	}
	return p.events, nil
}

func (p *plainParser) add(e rawEvent, raw string) {
	p.pending = append(p.pending, plainLine{e: e, raw: raw})
}

func (p *plainParser) output(test, text string) {
	p.add(rawEvent{Action: actionOutput, Test: test, Output: text + "\n"}, text)
}

func (p *plainParser) line(text string) {
	if m := plainBuild.FindStringSubmatch(text); m != nil {
		p.build = m[1]
		p.builds[displayPackage(m[1])] = m[1]
		p.events = append(p.events, rawEvent{Action: actionBuildOutput, ImportPath: m[1], Output: text + "\n"})
		return
	}
	summary := plainSummary.FindStringSubmatch(text)
	frame := plainFrame.FindStringSubmatch(text)
	result := plainResult.FindStringSubmatch(text)
	if p.build != "" && summary == nil && frame == nil && result == nil && text != "" {
		p.events = append(p.events, rawEvent{Action: actionBuildOutput, ImportPath: p.build, Output: text + "\n"})
		return
	}
	p.build = ""
	switch {
	case summary != nil:
		p.found = true
		p.summary(summary[1], summary[2], summary[3], text)
	case frame != nil:
		p.found = true
		p.current = frame[2]
		if frame[1] == "RUN" {
			p.add(rawEvent{Action: actionRun, Test: frame[2]}, text)
		}
	case result != nil:
		p.found = true
		p.add(rawEvent{Action: action(strings.ToLower(result[1])), Test: result[2], Elapsed: seconds(result[3])}, text)
		p.current = result[2]
	case summaryLine.MatchString(strings.TrimSpace(text)):
		p.output("", text)
	default:
		p.output(p.current, text)
	}
}

func (p *plainParser) summary(word, pkg, rest, text string) {
	for _, l := range p.pending {
		l.e.Package = pkg
		p.events = append(p.events, l.e)
	}
	p.pending, p.current = nil, ""
	p.events = append(p.events, rawEvent{Action: actionOutput, Package: pkg, Output: text + "\n"})
	e := rawEvent{Package: pkg}
	switch word {
	case "ok  ":
		e.Action = actionPass
	case "?   ":
		e.Action = actionSkip
	default:
		e.Action = actionFail
		if strings.Contains(rest, "[build failed]") {
			e.FailedBuild = p.builds[pkg]
			if e.FailedBuild == "" {
				e.FailedBuild = pkg
			}
			p.events = append(p.events, rawEvent{Action: actionBuildFail, ImportPath: e.FailedBuild})
		}
	}
	if m := plainElapsed.FindStringSubmatch(rest); m != nil {
		e.Elapsed = seconds(m[1])
	}
	p.events = append(p.events, e)
}

func seconds(s string) *float64 {
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &n
}

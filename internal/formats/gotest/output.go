package gotest

import (
	"slices"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/sanitize"
)

func splitOutput(s string) []string {
	return slices.Collect(core.SplitLines(strings.TrimSuffix(s, "\n")))
}

func (b *builder) isFrame(l line) bool {
	if b.outputType {
		return l.typ == "frame"
	}
	return frameLine.MatchString(l.text)
}

func (b *builder) testOutput(t *testState) output {
	var all []line
	errorLines := false
	for _, l := range t.lines {
		if b.isFrame(l) {
			continue
		}
		all = append(all, l)
		if l.typ == "error" || l.typ == "error-continue" {
			errorLines = true
		}
	}
	if b.o.Full || !b.outputType || !errorLines {
		return b.output(all, t.pkg)
	}
	var kept []line
	logs := 0
	panicking, racing := false, false
	for _, l := range all {
		text := strings.TrimSpace(l.text)
		if isPanicStart(l.text) {
			panicking = true
		}
		switch {
		case text == "WARNING: DATA RACE":
			racing = true
		case racing && raceSeparator(text):
			racing = false
			continue
		}
		switch {
		case raceSeparator(text):
		case panicking || racing || l.typ == "error" || l.typ == "error-continue":
			kept = append(kept, l)
		case text != "":
			logs++
		}
	}
	out := b.output(kept, t.pkg)
	out.LogsHidden = logs
	return out
}

func raceSeparator(s string) bool {
	return len(s) >= 10 && strings.Trim(s, "=") == ""
}

func isPanicStart(s string) bool {
	return strings.HasPrefix(s, "panic: ") || strings.HasPrefix(s, "fatal error: ")
}

func (b *builder) output(lines []line, pkg string) output {
	raw := make([]string, 0, len(lines))
	for _, l := range lines {
		raw = append(raw, l.text)
	}
	var out output
	if !b.o.Full {
		raw, out.StackHidden = filterStack(raw, pkg)
	}
	var cleaned []string
	for _, l := range raw {
		if c := sanitize.Line(l); strings.TrimSpace(c) != "" {
			cleaned = append(cleaned, c)
		}
	}
	out.Block = core.HeadBlock(core.Dedent(cleaned), b.o.MaxOutput)
	return out
}

func toLines(texts []string) []line {
	out := make([]line, 0, len(texts))
	for _, t := range texts {
		out = append(out, line{text: t})
	}
	return out
}

package generic

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/cauebraz/squeeze/internal/logscan"
	"github.com/cauebraz/squeeze/internal/sanitize"
)

type line struct {
	text    string
	isError bool
	warning bool
}

func (l line) flagged() bool { return l.isError || l.warning }

type parsed struct {
	lines        []line
	jsonIndented bool
}

func parse(input []byte, maxLineBytes int) parsed {
	var p parsed
	if indented, ok := indentJSON(input, maxLineBytes); ok {
		input, p.jsonIndented = indented, true
	}
	texts := splitLines(string(input))
	p.lines = make([]line, len(texts))
	for i, t := range texts {
		p.lines[i] = line{text: t, isError: logscan.ErrorLine(t), warning: logscan.WarningLine(t)}
	}
	return p
}

func indentJSON(input []byte, maxBytes int) ([]byte, bool) {
	trimmed := bytes.TrimSpace(input)
	if maxBytes <= 0 || len(trimmed) <= maxBytes || (trimmed[0] != '{' && trimmed[0] != '[') || bytes.IndexByte(trimmed, '\n') >= 0 || !json.Valid(trimmed) {
		return nil, false
	}
	var b bytes.Buffer
	if err := json.Indent(&b, trimmed, "", ""); err != nil {
		return nil, false
	}
	return b.Bytes(), true
}

func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	raw := strings.Split(s, "\n")
	out := make([]string, len(raw))
	for i, l := range raw {
		frames := strings.Split(strings.TrimSuffix(l, "\r"), "\r")
		for j := len(frames) - 1; j >= 0; j-- {
			if out[i] = sanitize.NarrowLine(frames[j]); strings.TrimSpace(out[i]) != "" {
				break
			}
		}
		if strings.TrimSpace(out[i]) == "" {
			out[i] = ""
		}
	}
	return out
}

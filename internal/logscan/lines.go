package logscan

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/cauebraz/squeeze/internal/sanitize"
)

type FlaggedLine struct {
	Text  string
	Error bool
}

var severityPrefix = regexp.MustCompile(`(?i)^(error|warning|warn)\s*:\s*`)

func FlaggedLines(text []byte) []FlaggedLine {
	var out []FlaggedLine
	for _, raw := range strings.Split(string(text), "\n") {
		l := sanitize.Clean(raw)
		isError := ErrorLine(l)
		if !isError && !WarningLine(l) {
			continue
		}
		if s := severityPrefix.ReplaceAllString(l, ""); s != "" {
			l = s
		}
		out = append(out, FlaggedLine{Text: l, Error: isError})
	}
	return out
}

func LeadingLines(input []byte) (lead, rest []byte) {
	for i := 0; i < len(input); {
		line, next := input[i:], len(input)
		if end := bytes.IndexByte(line, '\n'); end >= 0 {
			line, next = line[:end], i+end+1
		}
		trimmed := bytes.TrimSpace(line)
		switch {
		case len(trimmed) == 0:
		case trimmed[0] == '{' || trimmed[0] == '[':
			return input[:i], input[i:]
		case !ErrorLine(string(trimmed)) && !WarningLine(string(trimmed)):
			return nil, input
		}
		i = next
	}
	return nil, input
}

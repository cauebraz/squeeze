package core

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
)

func DecodeJSON(input []byte, v any) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(input))
	if err := dec.Decode(v); err != nil {
		return nil, err
	}
	return input[dec.InputOffset():], nil
}

type FlaggedLine struct {
	Text  string
	Error bool
}

var severityPrefix = regexp.MustCompile(`(?i)^(error|warning|warn)\s*:\s*`)

func FlaggedLines(text []byte) []FlaggedLine {
	var out []FlaggedLine
	for _, raw := range strings.Split(string(text), "\n") {
		l := Clean(raw)
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

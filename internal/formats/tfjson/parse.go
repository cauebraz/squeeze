package tfjson

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/cauebraz/squeeze/internal/jsonl"
)

var errNoStream = errors.New("no Terraform JSON stream lines in the input")

type inputLine struct {
	rawHeader
	rawPayload
	Text      string
	Malformed bool
}

func parse(input []byte) ([]inputLine, error) {
	var lines []inputLine
	stream := 0
	for _, raw := range jsonl.Lines(input) {
		l := parseLine(raw)
		if l.Type != "" {
			stream++
		}
		lines = append(lines, l)
	}
	if stream == 0 {
		return nil, errNoStream
	}
	return lines, nil
}

func parseLine(raw []byte) inputLine {
	if raw[0] != '{' {
		return inputLine{Text: string(raw)}
	}
	var l inputLine
	err := json.Unmarshal(raw, &struct {
		*rawHeader
		*rawPayload
	}{&l.rawHeader, &l.rawPayload})
	if err == nil {
		if l.Type == "" {
			return inputLine{Text: string(raw)}
		}
		return l
	}
	var h rawHeader
	if json.Unmarshal(raw, &h) != nil || h.Type == "" {
		return inputLine{Text: string(raw)}
	}
	return inputLine{rawHeader: h, Malformed: true}
}

func isStreamLine(raw []byte) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return false
	}
	var h rawHeader
	return json.Unmarshal(raw, &h) == nil && h.Type != "" && h.Level != ""
}

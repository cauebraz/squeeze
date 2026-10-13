package tfjson

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

var ErrNoStream = errors.New("no Terraform JSON stream lines in the input")

type Line struct {
	lineHeader
	linePayload
	Text      string
	Malformed bool
}

func Parse(input []byte) ([]Line, error) {
	r := bufio.NewReader(bytes.NewReader(input))
	var lines []Line
	stream := 0
	for {
		raw, err := r.ReadBytes('\n')
		if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 {
			l := parseLine(trimmed)
			if l.Type != "" {
				stream++
			}
			lines = append(lines, l)
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	if stream == 0 {
		return nil, ErrNoStream
	}
	return lines, nil
}

func parseLine(raw []byte) Line {
	if raw[0] != '{' {
		return Line{Text: string(raw)}
	}
	var h lineHeader
	if err := json.Unmarshal(raw, &h); err != nil || h.Type == "" {
		return Line{Text: string(raw)}
	}
	l := Line{lineHeader: h}
	if err := json.Unmarshal(raw, &l.linePayload); err != nil {
		l.linePayload = linePayload{}
		l.Malformed = true
	}
	return l
}

func isStreamLine(raw []byte) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return false
	}
	var h lineHeader
	return json.Unmarshal(raw, &h) == nil && h.Type != "" && h.Level != ""
}

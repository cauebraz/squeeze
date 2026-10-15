package golangci

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrNotGolangci = errors.New("input is not golangci-lint JSON")

type Position struct {
	Filename string `json:"Filename"`
	Line     int    `json:"Line"`
	Column   int    `json:"Column"`
}

type Issue struct {
	FromLinter     string            `json:"FromLinter"`
	Text           string            `json:"Text"`
	Severity       string            `json:"Severity"`
	Pos            Position          `json:"Pos"`
	SuggestedFixes []json.RawMessage `json:"SuggestedFixes"`
	Replacement    *json.RawMessage  `json:"Replacement"`
}

type Warning struct {
	Tag  string `json:"Tag"`
	Text string `json:"Text"`
}

type Linter struct {
	Name    string `json:"Name"`
	Enabled bool   `json:"Enabled"`
}

type Data struct {
	Warnings []Warning `json:"Warnings"`
	Linters  []Linter  `json:"Linters"`
	Error    string    `json:"Error"`
}

type Output struct {
	Issues   []Issue
	Report   Data
	Trailing []string
}

var statsLine = regexp.MustCompile(`^(\d+ issues:|0 issues\.|\* \S+: \d+)$`)

func Parse(input []byte) (*Output, error) {
	dec := json.NewDecoder(bytes.NewReader(input))
	var doc struct {
		Issues *[]Issue `json:"Issues"`
		Report *Data    `json:"Report"`
	}
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if doc.Issues == nil {
		return nil, ErrNotGolangci
	}
	out := &Output{Issues: *doc.Issues}
	if doc.Report != nil {
		out.Report = *doc.Report
	}
	for _, line := range strings.Split(string(input[dec.InputOffset():]), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !statsLine.MatchString(line) {
			out.Trailing = append(out.Trailing, line)
		}
	}
	return out, nil
}

package gotest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

var ErrNoEvents = errors.New("no go test -json events in the input")

var knownActions = map[string]bool{
	"start": true, "run": true, "pause": true, "cont": true, "pass": true, "bench": true,
	"fail": true, "output": true, "skip": true, "attr": true, "artifacts": true,
	"build-output": true, "build-fail": true,
}

type Event struct {
	Action      string   `json:"Action"`
	Package     string   `json:"Package"`
	Test        string   `json:"Test"`
	Elapsed     *float64 `json:"Elapsed"`
	Output      string   `json:"Output"`
	OutputType  string   `json:"OutputType"`
	FailedBuild string   `json:"FailedBuild"`
	ImportPath  string   `json:"ImportPath"`
	Key         string   `json:"Key"`
	Value       string   `json:"Value"`
	Path        string   `json:"Path"`
	Text        string   `json:"-"`
}

func Parse(input []byte) ([]Event, error) {
	r := bufio.NewReader(bytes.NewReader(input))
	var events []Event
	seen := 0
	for {
		raw, err := r.ReadBytes('\n')
		if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 {
			e := parseLine(trimmed)
			if e.Action != "" {
				seen++
			}
			events = append(events, e)
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	if seen == 0 {
		return nil, ErrNoEvents
	}
	return events, nil
}

func parseLine(raw []byte) Event {
	if raw[0] == '{' {
		var e Event
		if json.Unmarshal(raw, &e) == nil && e.Action != "" {
			return e
		}
	}
	return Event{Text: string(raw)}
}

func isEvent(raw []byte) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return false
	}
	var e Event
	if json.Unmarshal(raw, &e) != nil || !knownActions[e.Action] {
		return false
	}
	return e.Package != "" || e.ImportPath != ""
}

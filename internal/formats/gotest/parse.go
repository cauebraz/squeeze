package gotest

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/cauebraz/squeeze/internal/jsonl"
)

var errNotGoTestJSON = errors.New("no go test -json events in the input")

type action string

const (
	actionStart       action = "start"
	actionRun         action = "run"
	actionPause       action = "pause"
	actionCont        action = "cont"
	actionPass        action = "pass"
	actionBench       action = "bench"
	actionFail        action = "fail"
	actionOutput      action = "output"
	actionSkip        action = "skip"
	actionAttr        action = "attr"
	actionArtifacts   action = "artifacts"
	actionBuildOutput action = "build-output"
	actionBuildFail   action = "build-fail"
)

func (a action) known() bool {
	switch a {
	case actionStart, actionRun, actionPause, actionCont, actionPass, actionBench, actionFail, actionOutput, actionSkip, actionAttr, actionArtifacts, actionBuildOutput, actionBuildFail:
		return true
	}
	return false
}

type rawEvent struct {
	Action      action   `json:"Action"`
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

func parseEvents(input []byte) ([]rawEvent, error) {
	var events []rawEvent
	seen := 0
	for _, raw := range jsonl.Lines(input) {
		e := parseLine(raw)
		if e.Action != "" {
			seen++
		}
		events = append(events, e)
	}
	if seen == 0 {
		return nil, errNotGoTestJSON
	}
	return events, nil
}

func parseLine(raw []byte) rawEvent {
	if raw[0] == '{' {
		var e rawEvent
		if json.Unmarshal(raw, &e) == nil && e.Action != "" {
			return e
		}
	}
	return rawEvent{Text: string(raw)}
}

func isEvent(raw []byte) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '{' {
		return false
	}
	var e rawEvent
	if json.Unmarshal(raw, &e) != nil || !e.Action.known() {
		return false
	}
	return e.Package != "" || e.ImportPath != ""
}

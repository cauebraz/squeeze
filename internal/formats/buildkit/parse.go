package buildkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/jsonl"
	"github.com/cauebraz/squeeze/internal/logscan"
	"github.com/cauebraz/squeeze/internal/sanitize"
)

var errNoBuild = errors.New("no BuildKit progress output in the input")

var (
	plainStart = regexp.MustCompile(`^#0 building with "|^#\d+ \[([^\]]* )?internal\] load (build definition|\.dockerignore|metadata for|remote build context|git source)`)
	rawStart   = regexp.MustCompile(`^\{"(vertexes|statuses|logs|warnings)":\[\{"`)
	plainLine  = regexp.MustCompile(`^#(\d+) (.*)$`)
	logLine    = regexp.MustCompile(`^\d+\.\d+(?: (.*))?$`)
	doneLine   = regexp.MustCompile(`^DONE( \d+(\.\d+)?s)?$`)
	builderRe  = regexp.MustCompile(`^building with "([^"]*)" instance`)
	location   = regexp.MustCompile(`^.+:\d+$`)
	summary    = regexp.MustCompile(`^\d+ warnings? found`)
)

const sourceRule = "--------------------"

type vertex struct {
	name     string
	shown    bool
	started  time.Time
	done     bool
	cached   bool
	canceled bool
	err      string
	logs     []string
	raw      []byte
}

type parsedWarning struct {
	vertex string
	warning
}

type parsed struct {
	progress string
	builder  string
	order    []string
	byKey    map[string]*vertex
	warnings []parsedWarning
	errors   []string
	location string
}

func detect(head []byte) bool {
	return jsonl.Detect(head, 3, nil, func(l []byte) bool {
		l = bytes.TrimSpace(l)
		return plainStart.Match(l) || rawStart.Match(l)
	})
}

func (b *parsed) vertexFor(key string) *vertex {
	v, ok := b.byKey[key]
	if !ok {
		v = &vertex{}
		b.byKey[key] = v
		b.order = append(b.order, key)
	}
	return v
}

func parse(input []byte) (*parsed, error) {
	b := &parsed{byKey: map[string]*vertex{}}
	var textLines []string
	events := false
	for raw := range core.SplitLines(string(input)) {
		if strings.HasPrefix(strings.TrimSpace(raw), "{") && b.event([]byte(raw)) {
			events = true
			continue
		}
		textLines = append(textLines, raw)
	}
	plain := b.plain(textLines)
	if !events && !plain {
		return nil, errNoBuild
	}
	b.progress = "plain"
	if events {
		b.progress = "rawjson"
	}
	b.finish()
	return b, nil
}

type rawVertex struct {
	Digest        string     `json:"digest"`
	Name          string     `json:"name"`
	Started       *time.Time `json:"started"`
	Completed     *time.Time `json:"completed"`
	Cached        bool       `json:"cached"`
	Error         string     `json:"error"`
	ProgressGroup *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"progressGroup"`
}

type rawStatus struct {
	Vertexes []rawVertex       `json:"vertexes"`
	Statuses []json.RawMessage `json:"statuses"`
	Logs     []struct {
		Vertex string `json:"vertex"`
		Data   []byte `json:"data"`
	} `json:"logs"`
	Warnings []struct {
		Vertex string   `json:"vertex"`
		Short  []byte   `json:"short"`
		Detail [][]byte `json:"detail"`
		URL    string   `json:"url"`
	} `json:"warnings"`
}

func (b *parsed) event(line []byte) bool {
	var s rawStatus
	if err := json.Unmarshal(line, &s); err != nil {
		return false
	}
	if len(s.Vertexes)+len(s.Statuses)+len(s.Logs)+len(s.Warnings) == 0 {
		return false
	}
	for _, rv := range s.Vertexes {
		key, name := rv.Digest, rv.Name
		if rv.ProgressGroup != nil {
			key, name = "group:"+rv.ProgressGroup.ID, rv.ProgressGroup.Name
		}
		v := b.vertexFor(key)
		if name != "" {
			v.name = sanitize.Clean(name)
		}
		if rv.Started != nil && (!v.shown || rv.Started.Before(v.started)) {
			v.shown, v.started = true, *rv.Started
		}
		v.done = v.done || rv.Completed != nil
		v.cached = v.cached || rv.Cached
		switch {
		case strings.HasSuffix(rv.Error, "context canceled"):
			v.canceled = true
		case rv.Error != "":
			v.err = sanitize.Clean(rv.Error)
		}
	}
	for _, l := range s.Logs {
		v := b.vertexFor(l.Vertex)
		v.raw = append(v.raw, l.Data...)
	}
	for _, w := range s.Warnings {
		detail := []string{}
		for _, d := range w.Detail {
			detail = append(detail, sanitize.Clean(string(d)))
		}
		b.warnings = append(b.warnings, parsedWarning{vertex: w.Vertex, warning: warning{Message: sanitize.Clean(string(w.Short)), Detail: detail, URL: sanitize.Clean(w.URL)}})
	}
	return true
}

func (b *parsed) plain(lines []string) bool {
	seen, blank, trailer := false, true, false
	tail, source, inSummary := false, 0, false
	var cur *vertex
	var listed []string
	for i, raw := range lines {
		if tail {
			tail = strings.TrimSpace(raw) != "------"
			continue
		}
		if source > 0 {
			if strings.TrimSpace(raw) == sourceRule {
				source--
			}
			continue
		}
		if strings.TrimSpace(raw) == "" {
			blank = true
			continue
		}
		if m := plainLine.FindStringSubmatch(raw); m != nil && !trailer {
			seen = true
			cur = b.plainStep(m[1], m[2], blank)
			blank = false
			continue
		}
		blank = false
		c := sanitize.Clean(raw)
		switch {
		case c == "------":
			tail, trailer = true, true
		case location.MatchString(c) && i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == sourceRule:
			b.location, source, trailer = c, 2, true
		case strings.HasPrefix(c, "ERROR:"):
			b.errors = append(b.errors, strings.TrimSpace(strings.TrimPrefix(c, "ERROR:")))
			trailer = true
		case strings.HasPrefix(c, "WARNING:"):
			b.warnings = append(b.warnings, parsedWarning{warning: warning{Message: strings.TrimSpace(strings.TrimPrefix(c, "WARNING:")), Detail: []string{}}})
		case summary.MatchString(c):
			inSummary, trailer = true, true
		case inSummary && strings.HasPrefix(c, "- "):
			listed = append(listed, strings.TrimPrefix(c, "- "))
		case cur != nil && !trailer:
			cur.logs = append(cur.logs, sanitize.Line(raw))
		case logscan.ErrorLine(c):
			b.errors = append(b.errors, c)
		case logscan.WarningLine(c):
			b.warnings = append(b.warnings, parsedWarning{warning: warning{Message: c, Detail: []string{}}})
		}
	}
	known := map[string]bool{}
	for _, w := range b.warnings {
		known[w.Message] = true
	}
	for _, msg := range listed {
		if !known[msg] {
			b.warnings = append(b.warnings, parsedWarning{warning: warning{Message: msg, Detail: []string{}}})
		}
	}
	return seen
}

func (b *parsed) plainStep(id, rest string, blank bool) *vertex {
	if id == "0" {
		if m := builderRe.FindStringSubmatch(rest); m != nil {
			b.builder = sanitize.Clean(m[1])
		}
		return nil
	}
	key := "#" + id
	if _, ok := b.byKey[key]; !ok {
		v := b.vertexFor(key)
		v.name, v.shown = sanitize.Clean(rest), true
		return v
	}
	v := b.byKey[key]
	if blank && sanitize.Clean(rest) == v.name {
		return v
	}
	switch {
	case doneLine.MatchString(rest):
		v.done = true
	case rest == "CACHED":
		v.cached = true
	case rest == "CANCELED":
		v.canceled = true
	case strings.HasPrefix(rest, "ERROR:"):
		v.err = sanitize.Clean(strings.TrimPrefix(rest, "ERROR:"))
	case strings.HasPrefix(rest, "WARN:"):
		b.warnings = append(b.warnings, parsedWarning{vertex: key, warning: warning{Message: sanitize.Clean(strings.TrimPrefix(rest, "WARN:")), Detail: []string{}}})
	case logLine.MatchString(rest):
		v.logs = append(v.logs, sanitize.Line(logLine.FindStringSubmatch(rest)[1]))
	}
	return v
}

func (b *parsed) finish() {
	for _, key := range b.order {
		v := b.byKey[key]
		if len(v.raw) == 0 {
			continue
		}
		logText := strings.TrimSuffix(string(v.raw), "\n")
		for _, l := range strings.Split(logText, "\n") {
			v.logs = append(v.logs, sanitize.Line(l))
		}
	}
	shown := b.order[:0]
	for _, key := range b.order {
		if b.byKey[key].shown {
			shown = append(shown, key)
		}
	}
	slices.SortStableFunc(shown, func(x, y string) int {
		return b.byKey[x].started.Compare(b.byKey[y].started)
	})
	b.order = shown
}

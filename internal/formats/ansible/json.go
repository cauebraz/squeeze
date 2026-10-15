package ansible

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

var (
	jsonKeys = []*regexp.Regexp{
		regexp.MustCompile(`"custom_stats":\s*\{`),
		regexp.MustCompile(`"global_custom_stats":\s*\{`),
		regexp.MustCompile(`"plays":\s*\[`),
	}
	jsonStart = regexp.MustCompile(`(?m)^\{`)
)

type jsonRun struct {
	Plays []struct {
		Play struct {
			Name string `json:"name"`
		} `json:"play"`
		Tasks []struct {
			Task struct {
				Name string `json:"name"`
				Path string `json:"path"`
			} `json:"task"`
			Hosts map[string]map[string]json.RawMessage `json:"hosts"`
		} `json:"tasks"`
	} `json:"plays"`
	Stats map[string]struct {
		OK          int `json:"ok"`
		Changed     int `json:"changed"`
		Unreachable int `json:"unreachable"`
		Failures    int `json:"failures"`
		Skipped     int `json:"skipped"`
		Rescued     int `json:"rescued"`
		Ignored     int `json:"ignored"`
	} `json:"stats"`
}

func detectJSON(head []byte) bool {
	loc := jsonStart.FindIndex(head)
	if loc == nil {
		return false
	}
	doc := head[loc[0]:]
	for _, k := range jsonKeys {
		if !k.Match(doc) {
			return false
		}
	}
	return true
}

func parseJSON(input []byte) (Run, error) {
	loc := jsonStart.FindIndex(input)
	p := &textParser{lines: strings.Split(string(input[:loc[0]]), "\n")}
	p.parse()
	dec := json.NewDecoder(bytes.NewReader(input[loc[0]:]))
	var doc jsonRun
	if err := dec.Decode(&doc); err != nil {
		return Run{}, err
	}
	rest := input[loc[0]+int(dec.InputOffset()):]
	tail := &textParser{lines: strings.Split(string(rest), "\n")}
	tail.parse()
	r := Run{Input: "json", Plays: len(doc.Plays), HasRecap: true}
	r.Errors = append(p.run.Errors, tail.run.Errors...)
	r.Warnings = append(p.run.Warnings, tail.run.Warnings...)
	seq := 0
	for _, play := range doc.Plays {
		for _, t := range play.Tasks {
			seq++
			hosts := make([]string, 0, len(t.Hosts))
			for h := range t.Hosts {
				hosts = append(hosts, h)
			}
			sort.Strings(hosts)
			for _, h := range hosts {
				base := Event{Play: core.Clean(play.Play.Name), Task: core.Clean(t.Task.Name), Path: core.Clean(t.Task.Path), Host: core.Clean(h), Seq: seq}
				r.Events = append(r.Events, jsonEvents(base, t.Hosts[h])...)
			}
		}
	}
	hosts := make([]string, 0, len(doc.Stats))
	for h := range doc.Stats {
		hosts = append(hosts, h)
	}
	sort.Strings(hosts)
	for _, h := range hosts {
		s := doc.Stats[h]
		r.Recap = append(r.Recap, HostRecap{
			Host: core.Clean(h), OK: s.OK, Changed: s.Changed, Unreachable: s.Unreachable,
			Failed: s.Failures, Skipped: s.Skipped, Rescued: s.Rescued, Ignored: s.Ignored,
		})
	}
	return r, nil
}

func isTrue(m map[string]json.RawMessage, key string) bool {
	return string(bytes.TrimSpace(m[key])) == "true"
}

func statusOf(m map[string]json.RawMessage) Status {
	switch {
	case isTrue(m, "unreachable"):
		return StatusUnreachable
	case isTrue(m, "failed"):
		return StatusFailed
	case isTrue(m, "skipped"):
		return StatusSkipped
	case isTrue(m, "changed"):
		return StatusChanged
	}
	return StatusOK
}

func jsonEvents(base Event, m map[string]json.RawMessage) []Event {
	base.Status = statusOf(m)
	if base.Status == StatusUnreachable {
		base.Detail = detailOf(m)
	}
	if base.Status != StatusFailed {
		return []Event{base}
	}
	var items []map[string]json.RawMessage
	_ = json.Unmarshal(m["results"], &items)
	var out []Event
	for _, it := range items {
		if !itemFailed(it) && !isTrue(it, "unreachable") {
			continue
		}
		e := base
		e.Status = StatusFailed
		if isTrue(it, "unreachable") {
			e.Status = StatusUnreachable
		}
		e.Item = core.Clean(itemLabel(it))
		e.Detail = detailOf(it)
		out = append(out, e)
	}
	if len(out) == 0 {
		base.Detail = detailOf(m)
		out = append(out, base)
	}
	return out
}

func itemFailed(m map[string]json.RawMessage) bool {
	if _, ok := m["failed"]; ok {
		return isTrue(m, "failed")
	}
	if v, ok := m["failed_when_result"]; ok {
		s := rawText(v)
		return s != "" && s != "false" && s != "0"
	}
	if rc := rawText(m["rc"]); rc != "" && rc != "0" {
		return true
	}
	_, ok := m["exception"]
	return ok
}

func itemLabel(m map[string]json.RawMessage) string {
	if l := rawText(m["_ansible_item_label"]); l != "" {
		return l
	}
	return rawText(m["item"])
}

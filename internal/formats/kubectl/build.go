package kubectl

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/sanitize"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type object struct {
	Kind       string   `json:"kind"`
	Namespace  string   `json:"namespace"`
	Name       string   `json:"name"`
	Unhealthy  bool     `json:"unhealthy"`
	Status     string   `json:"status"`
	Details    []string `json:"details"`
	Restarts   int      `json:"restarts"`
	Containers []string `json:"containers,omitempty"`
	Keys       []string `json:"keys,omitempty"`
	Message    string   `json:"message,omitempty"`
}

type event struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Count     int    `json:"count"`
	Last      string `json:"last"`
}

type report struct {
	Tool            string              `json:"tool"`
	ToolVersion     string              `json:"tool_version"`
	ObjectsTotal    int                 `json:"objects_total"`
	Unhealthy       int                 `json:"unhealthy"`
	Restarts        int                 `json:"restarts"`
	Kinds           []core.Count        `json:"kinds"`
	Objects         []object            `json:"objects"`
	ObjectsOmitted  int                 `json:"objects_omitted"`
	UnhealthyCut    int                 `json:"unhealthy_omitted"`
	OmittedByKind   []core.Count        `json:"omitted_by_kind"`
	EventsTotal     int                 `json:"events_total"`
	Warnings        int                 `json:"warnings"`
	Events          []event             `json:"events"`
	EventsOmitted   int                 `json:"events_omitted"`
	OmittedByReason []core.Count        `json:"omitted_by_reason"`
	Diagnostics     []tfdiag.Diagnostic `json:"diagnostics,omitempty"`
}

type eventType string

const (
	eventNormal  eventType = "Normal"
	eventWarning eventType = "Warning"
)

type rawEvent struct {
	Type           eventType `json:"type"`
	Reason         string    `json:"reason"`
	Message        string    `json:"message"`
	Note           string    `json:"note"`
	Count          int       `json:"count"`
	DeprecatedCnt  int       `json:"deprecatedCount"`
	LastTimestamp  string    `json:"lastTimestamp"`
	DeprecatedLast string    `json:"deprecatedLastTimestamp"`
	EventTime      string    `json:"eventTime"`
	FirstTimestamp string    `json:"firstTimestamp"`
	Series         *struct {
		Count            int    `json:"count"`
		LastObservedTime string `json:"lastObservedTime"`
	} `json:"series"`
	InvolvedObject *rawRef `json:"involvedObject"`
	Regarding      *rawRef `json:"regarding"`
}

type rawRef struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func first(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func toEvent(r rawObject) (rawEvent, event, error) {
	var f rawEvent
	if err := json.Unmarshal(r.Doc, &f); err != nil {
		return rawEvent{}, event{}, fmt.Errorf("decode %s %s: %w", r.Kind, r.Meta.Name, err)
	}
	e := event{Reason: f.Reason, Message: first(f.Message, f.Note), Count: max(f.Count, f.DeprecatedCnt, 1)}
	last := ""
	if f.Series != nil {
		e.Count = max(e.Count, f.Series.Count)
		last = f.Series.LastObservedTime
	}
	e.Last = first(last, f.LastTimestamp, f.DeprecatedLast, f.EventTime, f.FirstTimestamp)
	about := f.InvolvedObject
	if about == nil {
		about = f.Regarding
	}
	if about != nil {
		e.Kind, e.Namespace, e.Name = about.Kind, about.Namespace, about.Name
	}
	return f, e, nil
}

func build(in document, o options) report {
	r := report{Tool: "kubectl", Kinds: []core.Count{}, Objects: []object{}, OmittedByKind: []core.Count{}, Events: []event{}, OmittedByReason: []core.Count{}}
	r.Diagnostics = tfdiag.Normalize(tfdiag.FromLines(in.Trailing, ""))
	var unhealthy, healthy []object
	var warnings []event
	kinds := map[string]int{}
	for _, raw := range in.Objects {
		if raw.Kind == "Event" {
			f, e, err := toEvent(raw)
			if err != nil {
				f.Type = eventWarning
				e = event{Kind: raw.Kind, Namespace: raw.Meta.Namespace, Name: raw.Meta.Name, Reason: "unreadable event", Count: 1}
			}
			e = e.clean()
			r.EventsTotal++
			if f.Type == eventWarning {
				r.Warnings++
				warnings = append(warnings, e)
			}
			continue
		}
		obj, err := toObject(raw)
		if err != nil {
			obj = object{Kind: raw.Kind, Namespace: raw.Meta.Namespace, Name: raw.Meta.Name, Unhealthy: true, Status: "unreadable status", Details: []string{}}
		}
		obj = obj.clean()
		kinds[obj.Kind]++
		r.ObjectsTotal++
		r.Restarts += obj.Restarts
		if obj.Unhealthy {
			r.Unhealthy++
			unhealthy = append(unhealthy, obj)
		} else {
			healthy = append(healthy, obj)
		}
	}
	r.Kinds = core.SortedCounts(kinds)
	all := slices.Concat(unhealthy, healthy)
	kept, _ := core.Limit(all, o.maxObjects)
	r.Objects = append(r.Objects, kept...)
	cut := map[string]int{}
	for _, obj := range all[len(kept):] {
		cut[obj.Kind]++
		r.ObjectsOmitted++
		if obj.Unhealthy {
			r.UnhealthyCut++
		}
	}
	r.OmittedByKind = core.SortedCounts(cut)
	events := fold(warnings)
	keptEvents, _ := core.Limit(events, o.maxEvents)
	r.Events = append(r.Events, keptEvents...)
	reasons := map[string]int{}
	for _, e := range events[len(keptEvents):] {
		reasons[first(e.Reason, "no reason")]++
		r.EventsOmitted++
	}
	r.OmittedByReason = core.SortedCounts(reasons)
	return r
}

func fold(events []event) []event {
	type key struct{ kind, ns, name, reason, message string }
	index := map[key]int{}
	var out []event
	for _, e := range events {
		k := key{e.Kind, e.Namespace, e.Name, e.Reason, e.Message}
		if i, ok := index[k]; ok {
			out[i].Count += e.Count
			if compareTimes(e.Last, out[i].Last) > 0 {
				out[i].Last = e.Last
			}
			continue
		}
		index[k] = len(out)
		out = append(out, e)
	}
	slices.SortStableFunc(out, func(a, b event) int { return compareTimes(b.Last, a.Last) })
	return out
}

func compareTimes(a, b string) int {
	ta, errA := time.Parse(time.RFC3339Nano, a)
	tb, errB := time.Parse(time.RFC3339Nano, b)
	return cmp.Or(cmp.Compare(parsed(errA), parsed(errB)), ta.Compare(tb), cmp.Compare(a, b))
}

func parsed(err error) int {
	if err != nil {
		return 0
	}
	return 1
}

func (o object) clean() object {
	o.Kind, o.Namespace, o.Name = sanitize.Clean(o.Kind), sanitize.Clean(o.Namespace), sanitize.Clean(o.Name)
	o.Status, o.Message = sanitize.Clean(o.Status), sanitize.Clean(o.Message)
	o.Details, o.Containers = cleanAll(o.Details), cleanAll(o.Containers)
	if o.Keys != nil {
		o.Keys = cleanAll(o.Keys)
		slices.Sort(o.Keys)
	}
	return o
}

func (e event) clean() event {
	e.Kind, e.Namespace, e.Name = sanitize.Clean(e.Kind), sanitize.Clean(e.Namespace), sanitize.Clean(e.Name)
	e.Reason, e.Message, e.Last = sanitize.Clean(e.Reason), sanitize.Clean(e.Message), sanitize.Clean(e.Last)
	return e
}

func cleanAll(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = sanitize.Clean(s)
	}
	return out
}

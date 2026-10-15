package kubectl

import (
	"encoding/json"
	"sort"

	"github.com/cauebraz/squeeze/internal/core"
)

type Object struct {
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

type Event struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Reason    string `json:"reason"`
	Message   string `json:"message"`
	Count     int    `json:"count"`
	Last      string `json:"last"`
}

type Report struct {
	Tool            string       `json:"tool"`
	ToolVersion     string       `json:"tool_version"`
	ObjectsTotal    int          `json:"objects_total"`
	Unhealthy       int          `json:"unhealthy"`
	Restarts        int          `json:"restarts"`
	Kinds           []core.Count `json:"kinds"`
	Objects         []Object     `json:"objects"`
	ObjectsOmitted  int          `json:"objects_omitted"`
	UnhealthyCut    int          `json:"unhealthy_omitted"`
	OmittedByKind   []core.Count `json:"omitted_by_kind"`
	EventsTotal     int          `json:"events_total"`
	Warnings        int          `json:"warnings"`
	Events          []Event      `json:"events"`
	EventsOmitted   int          `json:"events_omitted"`
	OmittedByReason []core.Count `json:"omitted_by_reason"`
}

type eventFields struct {
	Type           string `json:"type"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	Note           string `json:"note"`
	Count          int    `json:"count"`
	DeprecatedCnt  int    `json:"deprecatedCount"`
	LastTimestamp  string `json:"lastTimestamp"`
	DeprecatedLast string `json:"deprecatedLastTimestamp"`
	EventTime      string `json:"eventTime"`
	FirstTimestamp string `json:"firstTimestamp"`
	Series         *struct {
		Count            int    `json:"count"`
		LastObservedTime string `json:"lastObservedTime"`
	} `json:"series"`
	InvolvedObject *ref `json:"involvedObject"`
	Regarding      *ref `json:"regarding"`
}

type ref struct {
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

func event(r Raw) (eventFields, Event) {
	var f eventFields
	raw, _ := json.Marshal(r.Fields)
	_ = json.Unmarshal(raw, &f)
	e := Event{Reason: f.Reason, Message: first(f.Message, f.Note), Count: max(f.Count, f.DeprecatedCnt, 1)}
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
	return f, e
}

func Build(raws []Raw, o Options) Report {
	r := Report{Tool: "kubectl", Kinds: []core.Count{}, Objects: []Object{}, OmittedByKind: []core.Count{}, Events: []Event{}, OmittedByReason: []core.Count{}}
	var unhealthy, healthy []Object
	var warnings []Event
	kinds := map[string]int{}
	for _, raw := range raws {
		if raw.Kind == "Event" {
			f, e := event(raw)
			r.EventsTotal++
			if f.Type == "Warning" {
				r.Warnings++
				warnings = append(warnings, e)
			}
			continue
		}
		obj, err := object(raw)
		if err != nil {
			obj = Object{Kind: raw.Kind, Namespace: raw.Meta.Namespace, Name: raw.Meta.Name, Unhealthy: true, Status: "unreadable status", Details: []string{}}
		}
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
	all := append(unhealthy, healthy...)
	kept, _ := core.Limit(all, o.MaxObjects)
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
	keptEvents, _ := core.Limit(events, o.MaxEvents)
	r.Events = append(r.Events, keptEvents...)
	reasons := map[string]int{}
	for _, e := range events[len(keptEvents):] {
		reasons[first(e.Reason, "no reason")]++
		r.EventsOmitted++
	}
	r.OmittedByReason = core.SortedCounts(reasons)
	return r
}

func fold(events []Event) []Event {
	type key struct{ kind, ns, name, reason, message string }
	index := map[key]int{}
	var out []Event
	for _, e := range events {
		k := key{e.Kind, e.Namespace, e.Name, e.Reason, e.Message}
		if i, ok := index[k]; ok {
			out[i].Count += e.Count
			if e.Last > out[i].Last {
				out[i].Last = e.Last
			}
			continue
		}
		index[k] = len(out)
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Last > out[j].Last })
	return out
}

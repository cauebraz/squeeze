package kubectl

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

const (
	maxMessage       = 200
	maxKindsInHeader = 5
	maxInNotice      = 5
)

func name(kind, namespace, n string) string {
	id := core.Clean(n)
	if namespace != "" {
		id = core.Clean(namespace) + "/" + id
	}
	if kind == "" {
		return id
	}
	return core.Clean(kind) + " " + id
}

func Text(r Report) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, o := range r.Objects {
		b.WriteString(line(o))
		b.WriteByte('\n')
	}
	if r.ObjectsOmitted > 0 {
		what := fmt.Sprintf("%d more %s", r.ObjectsOmitted, core.Plural(r.ObjectsOmitted, "object", "objects"))
		if r.UnhealthyCut > 0 {
			what += fmt.Sprintf(", %d unhealthy", r.UnhealthyCut)
		}
		fmt.Fprintf(&b, "... %s: %s (--max-objects 0)\n", what, core.TopCounts(clean(r.OmittedByKind), maxInNotice, "kind", "kinds"))
	}
	for _, e := range r.Events {
		b.WriteString("warning ")
		if about := name(e.Kind, e.Namespace, e.Name); about != "" {
			b.WriteString(about + " ")
		}
		b.WriteString(core.Clean(first(e.Reason, "no reason")))
		if e.Message != "" {
			fmt.Fprintf(&b, ": %s", core.Ellipsis(core.Clean(e.Message), maxMessage))
		}
		if e.Count > 1 {
			fmt.Fprintf(&b, " (x%d)", e.Count)
		}
		b.WriteByte('\n')
	}
	if r.EventsOmitted > 0 {
		fmt.Fprintf(&b, "... %d more warning %s: %s (--max-events 0)\n", r.EventsOmitted, core.Plural(r.EventsOmitted, "event", "events"), core.TopCounts(clean(r.OmittedByReason), maxInNotice, "reason", "reasons"))
	}
	return b.String()
}

func header(r Report) string {
	var parts []string
	if r.ObjectsTotal > 0 || r.EventsTotal == 0 {
		objects := fmt.Sprintf("%d %s", r.ObjectsTotal, core.Plural(r.ObjectsTotal, "object", "objects"))
		if r.ObjectsTotal > 0 {
			objects += fmt.Sprintf(" (%s), %d unhealthy", core.TopCounts(clean(r.Kinds), maxKindsInHeader, "kind", "kinds"), r.Unhealthy)
		}
		parts = append(parts, objects)
	}
	if r.EventsTotal > 0 {
		parts = append(parts, fmt.Sprintf("%d %s, %d %s", r.EventsTotal, core.Plural(r.EventsTotal, "event", "events"), r.Warnings, core.Plural(r.Warnings, "warning", "warnings")))
	}
	return "kubectl: " + strings.Join(parts, "; ")
}

func line(o Object) string {
	var b strings.Builder
	if o.Unhealthy {
		b.WriteString("unhealthy ")
	}
	b.WriteString(name(o.Kind, o.Namespace, o.Name))
	var parts []string
	if o.Status != "" {
		parts = append(parts, core.Clean(o.Status))
	}
	for _, d := range o.Details {
		parts = append(parts, core.Clean(d))
	}
	if len(o.Keys) > 0 {
		keys := make([]string, len(o.Keys))
		for i, k := range o.Keys {
			keys[i] = core.Clean(k)
		}
		parts = append(parts, "keys "+strings.Join(keys, " "))
	}
	if len(parts) > 0 {
		b.WriteString(": ")
		b.WriteString(strings.Join(parts, ", "))
	}
	for _, c := range o.Containers {
		b.WriteString("; ")
		b.WriteString(core.Clean(c))
	}
	if o.Message != "" {
		b.WriteString("; ")
		b.WriteString(core.Ellipsis(core.Clean(o.Message), maxMessage))
	}
	return b.String()
}

func clean(counts []core.Count) []core.Count {
	out := make([]core.Count, len(counts))
	for i, c := range counts {
		out[i] = core.Count{Name: core.NoticeName(core.Clean(c.Name)), Count: c.Count}
	}
	return out
}

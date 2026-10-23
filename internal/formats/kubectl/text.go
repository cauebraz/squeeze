package kubectl

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const (
	maxMessage       = 200
	maxKindsInHeader = 5
	maxInNotice      = 5
)

func name(kind, namespace, n string) string {
	id := n
	if namespace != "" {
		id = namespace + "/" + id
	}
	if kind == "" {
		return id
	}
	return kind + " " + id
}

func text(r report) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, d := range r.Diagnostics {
		tfdiag.Write(&b, d, tfdiag.WriteOptions{})
	}
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
		b.WriteString(first(e.Reason, "no reason"))
		if e.Message != "" {
			fmt.Fprintf(&b, ": %s", core.Ellipsis(e.Message, maxMessage))
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

func header(r report) string {
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

func line(o object) string {
	var b strings.Builder
	if o.Unhealthy {
		b.WriteString("unhealthy ")
	}
	b.WriteString(name(o.Kind, o.Namespace, o.Name))
	var parts []string
	if o.Status != "" {
		parts = append(parts, o.Status)
	}
	parts = append(parts, o.Details...)
	if len(o.Keys) > 0 {
		parts = append(parts, "keys "+strings.Join(o.Keys, " "))
	}
	if len(parts) > 0 {
		b.WriteString(": ")
		b.WriteString(strings.Join(parts, ", "))
	}
	for _, c := range o.Containers {
		b.WriteString("; ")
		b.WriteString(c)
	}
	if o.Message != "" {
		b.WriteString("; ")
		b.WriteString(core.Ellipsis(o.Message, maxMessage))
	}
	return b.String()
}

func clean(counts []core.Count) []core.Count {
	out := make([]core.Count, len(counts))
	for i, c := range counts {
		out[i] = core.Count{Name: core.NoticeName(c.Name), Count: c.Count}
	}
	return out
}

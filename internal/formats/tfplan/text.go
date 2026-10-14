package tfplan

import (
	"encoding/json"
	"fmt"
	"github.com/cauebraz/squeeze/internal/core"
	"strconv"
	"strings"
)

const maxValueBytes = 48

func Text(r Report, o Options) string {
	var b strings.Builder
	if r.Summary.Errored {
		b.WriteString("plan errored\n")
	}
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, c := range r.Changes {
		writeResource(&b, c)
	}
	if r.Omitted > 0 {
		fmt.Fprintf(&b, "... %d more %s (--max-resources 0)\n", r.Omitted, core.Plural(r.Omitted, "resource", "resources"))
	}
	if r.NoopHidden > 0 {
		fmt.Fprintf(&b, "... %d %s without changes (--all)\n", r.NoopHidden, core.Plural(r.NoopHidden, "resource", "resources"))
	}
	writeChecks(&b, r.Checks)
	writeDrift(&b, r, o.ListDrift)
	writeDeferred(&b, r.Deferred)
	writeActions(&b, r.Actions)
	writeOutputs(&b, r.Outputs)
	return b.String()
}

func header(r Report) string {
	s := r.Summary
	tool := r.Tool
	if r.ToolVersion != "" {
		tool += " " + r.ToolVersion
	}
	var parts []string
	if s.Changed() {
		if s.Import > 0 {
			parts = append(parts, fmt.Sprintf("%d to import", s.Import))
		}
		parts = append(parts,
			fmt.Sprintf("%d to add", s.Add),
			fmt.Sprintf("%d to change", s.Change),
			fmt.Sprintf("%d to destroy", s.Destroy),
		)
		optional := []struct {
			n    int
			verb string
		}{
			{s.Replace, "to replace"},
			{s.Forget, "to forget"},
			{s.Move, "to move"},
			{s.Read, "to read"},
		}
		for _, p := range optional {
			if p.n > 0 {
				parts = append(parts, fmt.Sprintf("%d %s", p.n, p.verb))
			}
		}
	} else {
		parts = append(parts, "no changes")
	}
	line := tool + " plan: " + strings.Join(parts, ", ")
	if s.Drift > 0 {
		line += fmt.Sprintf("; %d drifted", s.Drift)
	}
	if s.Deferred > 0 {
		line += fmt.Sprintf("; %d deferred", s.Deferred)
	}
	if s.Actions > 0 {
		line += fmt.Sprintf("; %d %s to invoke", s.Actions, core.Plural(s.Actions, "action", "actions"))
	}
	if s.Outputs > 0 {
		line += fmt.Sprintf("; %d %s changed", s.Outputs, core.Plural(s.Outputs, "output", "outputs"))
	}
	if s.ChecksFailed > 0 {
		line += fmt.Sprintf("; %d failed %s", s.ChecksFailed, core.Plural(s.ChecksFailed, "check", "checks"))
	}
	if !s.Complete {
		line += "; incomplete"
	}
	return line
}

func writeChecks(b *strings.Builder, checks []CheckResult) {
	if len(checks) == 0 {
		return
	}
	b.WriteString("checks:\n")
	for _, c := range checks {
		b.WriteString(c.Status)
		b.WriteByte(' ')
		b.WriteString(c.Address)
		if len(c.Problems) > 0 {
			b.WriteString(": ")
			b.WriteString(strings.Join(c.Problems, "; "))
		}
		b.WriteByte('\n')
	}
}

func writeResource(b *strings.Builder, c ResourceChange) {
	b.WriteString(c.Action.Symbol())
	b.WriteByte(' ')
	b.WriteString(c.Address)
	if n := notes(c); len(n) > 0 {
		b.WriteString(" (")
		b.WriteString(strings.Join(n, "; "))
		b.WriteByte(')')
	}
	b.WriteByte('\n')
	for _, a := range c.Attrs {
		b.WriteString("  ")
		b.WriteString(attrLine(a))
		b.WriteByte('\n')
	}
	if c.AttrsOmitted > 0 {
		fmt.Fprintf(b, "  ... %d more %s (--max-attrs 0)\n", c.AttrsOmitted, core.Plural(c.AttrsOmitted, "attribute", "attributes"))
	}
}

func notes(c ResourceChange) []string {
	var n []string
	if c.Deposed != "" {
		n = append(n, "deposed object "+c.Deposed)
	}
	if c.PreviousAddress != "" && c.PreviousAddress != c.Address {
		n = append(n, "moved from "+c.PreviousAddress)
	}
	if c.Importing && c.Action != Import {
		n = append(n, "import")
	}
	switch c.Action {
	case Forget:
		n = append(n, "removed from state, not destroyed")
	case ForgetThenCreate, CreateThenForget:
		n = append(n, "existing object not destroyed")
	case Noop, Create, Read, Update, Delete, Replace, ReplaceCBD, Import:
	}
	if len(c.ReplacedBy) > 0 {
		n = append(n, "replacement forced by "+strings.Join(c.ReplacedBy, ", "))
	}
	if c.Reason != "" {
		n = append(n, c.Reason)
	}
	if c.Irrelevant {
		n = append(n, "does not affect this plan")
	}
	return n
}

func renderScalar(s *Scalar) (string, bool) {
	if s == nil {
		return "", false
	}
	var out string
	switch v := s.Value.(type) {
	case nil:
		out = "null"
	case bool:
		out = strconv.FormatBool(v)
	case string:
		out = strconv.Quote(v)
	case json.Number:
		out = v.String()
	case float64:
		out = strconv.FormatFloat(v, 'g', -1, 64)
	default:
		return "", false
	}
	return out, len(out) <= maxValueBytes
}

func attrLine(a AttrChange) string {
	sym := a.Kind.Symbol()
	if a.ForcesReplacement {
		sym = "!"
	}
	prefix := sym + " " + a.Path
	before, hasBefore := renderScalar(a.Before)
	after, hasAfter := renderScalar(a.After)
	switch {
	case a.Reordered:
		return prefix + ": order changed"
	case a.Sensitive && a.Unknown:
		return prefix + ": (sensitive, known after apply)"
	case a.Sensitive:
		return prefix + ": (sensitive)"
	case a.Unknown && hasBefore:
		return prefix + ": " + before + " -> (known after apply)"
	case a.Unknown:
		return prefix + ": (known after apply)"
	case a.Kind == Added && hasAfter:
		return prefix + ": " + after
	case a.Kind == Removed && hasBefore:
		return prefix + ": " + before
	case a.Kind == Modified && hasBefore && hasAfter:
		return prefix + ": " + before + " -> " + after
	default:
		return prefix
	}
}

func writeDrift(b *strings.Builder, r Report, list bool) {
	if n := len(r.Drift); n > 0 {
		if list {
			b.WriteString("drift:\n")
			for _, c := range r.Drift {
				writeResource(b, c)
			}
		} else {
			fmt.Fprintf(b, "... %d drifted %s (--drift)\n", n, core.Plural(n, "resource", "resources"))
		}
	}
	if r.DriftOmitted > 0 {
		fmt.Fprintf(b, "... %d drifted %s not affecting this plan (--all)\n", r.DriftOmitted, core.Plural(r.DriftOmitted, "resource", "resources"))
	}
}

func writeDeferred(b *strings.Builder, deferred []DeferredChange) {
	if len(deferred) == 0 {
		return
	}
	b.WriteString("deferred:\n")
	for _, d := range deferred {
		fmt.Fprintf(b, "%s %s (%s)\n", d.Action.Symbol(), d.Address, d.Reason)
	}
}

func writeActions(b *strings.Builder, actions []ActionInvocation) {
	if len(actions) == 0 {
		return
	}
	items := make([]string, 0, len(actions))
	for _, a := range actions {
		item := a.Address
		switch {
		case a.Event != "" && a.Resource != "":
			item += " (" + a.Event + " " + a.Resource + ")"
		case a.Event != "":
			item += " (" + a.Event + ")"
		case a.Invoked:
			item += " (invoked)"
		}
		items = append(items, item)
	}
	b.WriteString("actions: ")
	b.WriteString(strings.Join(items, ", "))
	b.WriteByte('\n')
}

func writeOutputs(b *strings.Builder, outputs []OutputChange) {
	if len(outputs) == 0 {
		return
	}
	items := make([]string, 0, len(outputs))
	for _, o := range outputs {
		item := o.Action.Symbol() + " " + o.Name
		switch {
		case o.Sensitive:
			item += " (sensitive)"
		case o.Unknown:
			item += " (known after apply)"
		}
		items = append(items, item)
	}
	b.WriteString("outputs: ")
	b.WriteString(strings.Join(items, ", "))
	b.WriteByte('\n')
}

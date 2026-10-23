package tfplan

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const maxValueBytes = 48

func text(r report, o options) string {
	var b strings.Builder
	if r.Summary.Errored {
		b.WriteString("plan errored\n")
	}
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, d := range r.Diagnostics {
		tfdiag.Write(&b, d, tfdiag.WriteOptions{})
	}
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
	writeDeferred(&b, r.Deferred, r.DeferredOmitted)
	tfdiag.WriteActions(&b, r.Actions)
	writeDeferredActions(&b, r.DeferredActions)
	writeOutputs(&b, r.Outputs)
	return b.String()
}

func header(r report) string {
	s := r.Summary
	tool := core.ToolLabel(r.Tool, r.ToolVersion)
	var parts []string
	if s.changed() {
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
			{s.Unknown, "with unknown actions"},
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
	if s.DeferredActions > 0 {
		line += fmt.Sprintf("; %d deferred %s", s.DeferredActions, core.Plural(s.DeferredActions, "action", "actions"))
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

func writeChecks(b *strings.Builder, checks []checkResult) {
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

func writeResource(b *strings.Builder, c resourceChange) {
	b.WriteString(c.Action.symbol())
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

func notes(c resourceChange) []string {
	var n []string
	if c.Deposed != "" {
		n = append(n, "deposed object "+c.Deposed)
	}
	if c.PreviousAddress != "" && c.PreviousAddress != c.Address {
		n = append(n, "moved from "+c.PreviousAddress)
	}
	if c.Importing && c.Action != actionImport {
		n = append(n, "import")
	}
	if c.GeneratedLines > 0 {
		g := fmt.Sprintf("%d %s of generated config", c.GeneratedLines, core.Plural(c.GeneratedLines, "line", "lines"))
		if c.GeneratedConfig != "" {
			g += " in --output json"
		}
		n = append(n, g)
	}
	switch c.Action {
	case actionForget:
		n = append(n, "removed from state, not destroyed")
	case actionForgetThenCreate, actionCreateThenForget:
		n = append(n, "existing object not destroyed")
	case actionUnknown:
		n = append(n, unknownNote(c.Actions))
	case actionNoop, actionCreate, actionRead, actionUpdate, actionDelete, actionReplace, actionReplaceCBD, actionImport:
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

func unknownNote(actions []string) string {
	return "unknown actions [" + strings.Join(actions, ", ") + "]"
}

func renderScalar(s *scalar) (string, bool) {
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

func attrLine(a attrChange) string {
	sym := a.Kind.symbol()
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
	case a.Kind == attrAdded && hasAfter:
		return prefix + ": " + after
	case a.Kind == attrRemoved && hasBefore:
		return prefix + ": " + before
	case a.Kind == attrModified && hasBefore && hasAfter:
		return prefix + ": " + before + " -> " + after
	default:
		return prefix
	}
}

func writeDrift(b *strings.Builder, r report, list bool) {
	if n := len(r.Drift) + r.DriftOmitted; n > 0 {
		if list {
			b.WriteString("drift:\n")
			for _, c := range r.Drift {
				writeResource(b, c)
			}
			if r.DriftOmitted > 0 {
				fmt.Fprintf(b, "... %d more drifted %s (--max-resources 0)\n", r.DriftOmitted, core.Plural(r.DriftOmitted, "resource", "resources"))
			}
		} else {
			fmt.Fprintf(b, "... %d drifted %s (--drift)\n", n, core.Plural(n, "resource", "resources"))
		}
	}
	if r.DriftHidden > 0 {
		fmt.Fprintf(b, "... %d drifted %s not affecting this plan (--all)\n", r.DriftHidden, core.Plural(r.DriftHidden, "resource", "resources"))
	}
}

func writeDeferred(b *strings.Builder, deferred []deferredChange, omitted int) {
	if len(deferred) == 0 {
		return
	}
	b.WriteString("deferred:\n")
	for _, d := range deferred {
		reason := d.Reason
		if d.Action == actionUnknown {
			reason += "; " + unknownNote(d.Actions)
		}
		fmt.Fprintf(b, "%s %s (%s)\n", d.Action.symbol(), d.Address, reason)
	}
	if omitted > 0 {
		fmt.Fprintf(b, "... %d more deferred %s (--max-resources 0)\n", omitted, core.Plural(omitted, "resource", "resources"))
	}
}

func writeDeferredActions(b *strings.Builder, deferred []deferredAction) {
	if len(deferred) == 0 {
		return
	}
	items := make([]string, 0, len(deferred))
	for _, d := range deferred {
		items = append(items, d.Item(d.Reason))
	}
	b.WriteString("deferred actions: ")
	b.WriteString(strings.Join(items, ", "))
	b.WriteByte('\n')
}

func writeOutputs(b *strings.Builder, outputs []outputChange) {
	if len(outputs) == 0 {
		return
	}
	items := make([]string, 0, len(outputs))
	for _, o := range outputs {
		item := o.Action.symbol() + " " + o.Name
		switch {
		case o.Action == actionUnknown:
			item += " (" + unknownNote(o.Actions) + ")"
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

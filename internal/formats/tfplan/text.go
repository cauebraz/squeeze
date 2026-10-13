package tfplan

import (
	"fmt"
	"strings"
)

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
		fmt.Fprintf(&b, "... %d more resources (--max-resources)\n", r.Omitted)
	}
	writeDrift(&b, r.Drift, o.ListDrift)
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
			{s.Import, "to import"},
			{s.Read, "to read"},
			{s.Forget, "to forget"},
			{s.Move, "to move"},
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
	if s.Outputs > 0 {
		line += fmt.Sprintf("; %d %s changed", s.Outputs, plural(s.Outputs, "output", "outputs"))
	}
	if !s.Complete {
		line += "; incomplete"
	}
	return line
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
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
		fmt.Fprintf(b, "  ... %d more attributes (--max-attrs)\n", c.AttrsOmitted)
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
	if c.Action == Forget {
		n = append(n, "removed from state, not destroyed")
	}
	if len(c.ReplacedBy) > 0 {
		n = append(n, "replacement forced by "+strings.Join(c.ReplacedBy, ", "))
	}
	if c.Reason != "" {
		n = append(n, c.Reason)
	}
	return n
}

func attrLine(a AttrChange) string {
	sym := "~"
	if a.ForcesReplacement {
		sym = "!"
	}
	prefix := sym + " " + a.Path
	switch {
	case a.Sensitive && a.Unknown:
		return prefix + ": (sensitive, known after apply)"
	case a.Sensitive:
		return prefix + ": (sensitive)"
	case a.Unknown && a.Before != "":
		return prefix + ": " + a.Before + " -> (known after apply)"
	case a.Unknown:
		return prefix + ": (known after apply)"
	case a.Before != "" && a.After != "":
		return prefix + ": " + a.Before + " -> " + a.After
	default:
		return prefix
	}
}

func writeDrift(b *strings.Builder, drift []ResourceChange, list bool) {
	if len(drift) == 0 {
		return
	}
	if !list {
		fmt.Fprintf(b, "drift: %d %s (--drift to list)\n", len(drift), plural(len(drift), "resource", "resources"))
		return
	}
	b.WriteString("drift:\n")
	for _, c := range drift {
		writeResource(b, c)
	}
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

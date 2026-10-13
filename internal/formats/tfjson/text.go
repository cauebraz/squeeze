package tfjson

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const (
	maxValueBytes   = 48
	maxMessageBytes = 200
)

var actionSymbols = map[string]string{
	"create": "+", "delete": "-", "update": "~", "replace": "-/+", "read": "<=",
	"remove": ".", "import": ">>", "move": "=", "noop": "=",
}

func symbol(action string) string {
	if s, ok := actionSymbols[action]; ok {
		return s
	}
	return "?"
}

func Text(r Report, o Options) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, c := range r.Changes {
		writeChange(&b, c)
	}
	if r.Omitted > 0 {
		fmt.Fprintf(&b, "... %d more %s (--max-resources)\n", r.Omitted, plural(r.Omitted, "resource", "resources"))
	}
	writeApplied(&b, r.Applied)
	for _, f := range r.Failures {
		b.WriteString("error: ")
		b.WriteString(f.Message)
		b.WriteByte('\n')
	}
	for _, p := range r.Policies {
		fmt.Fprintf(&b, "policy: %s %s", p.Policy, p.Result)
		if p.Target != "" {
			b.WriteString(" for " + p.Target)
		}
		if p.Enforcement != "" {
			b.WriteString(" (" + p.Enforcement + ")")
		}
		b.WriteByte('\n')
	}
	for _, d := range r.Diagnostics {
		tfdiag.Write(&b, d, tfdiag.WriteOptions{NoDetail: o.NoDetail})
	}
	if r.DiagnosticsOmitted > 0 {
		fmt.Fprintf(&b, "... %d more %s (--max-diagnostics)\n", r.DiagnosticsOmitted, plural(r.DiagnosticsOmitted, "diagnostic", "diagnostics"))
	}
	writeDrift(&b, r.Drift, o.ListDrift)
	if len(r.Deferred) > 0 {
		b.WriteString("deferred:\n")
		for _, d := range r.Deferred {
			fmt.Fprintf(&b, "%s %s (%s)\n", symbol(d.Action), d.Address, d.Reason)
		}
	}
	writeOutputs(&b, r.Outputs)
	writeTests(&b, r.Tests, o)
	writeOther(&b, r, o.All)
	return b.String()
}

func header(r Report) string {
	line := r.Tool
	if r.ToolVersion != "" {
		line += " " + r.ToolVersion
	}
	if r.Operation != "" {
		line += " " + r.Operation
	}
	var segments []string
	if words := summaryWords(r); words != "" {
		segments = append(segments, words)
	}
	if n := len(r.Drift); n > 0 {
		segments = append(segments, fmt.Sprintf("%d drifted", n))
	}
	if n := len(r.Deferred); n > 0 {
		segments = append(segments, fmt.Sprintf("%d deferred", n))
	}
	var diags []string
	if r.Errors > 0 {
		diags = append(diags, fmt.Sprintf("%d %s", r.Errors, plural(r.Errors, "error", "errors")))
	}
	if r.Warnings > 0 {
		diags = append(diags, fmt.Sprintf("%d %s", r.Warnings, plural(r.Warnings, "warning", "warnings")))
	}
	if len(diags) > 0 {
		segments = append(segments, strings.Join(diags, ", "))
	}
	if len(segments) > 0 {
		line += ": " + strings.Join(segments, "; ")
	}
	return line
}

func summaryWords(r Report) string {
	if s := r.Summary; s != nil {
		if r.Operation == "apply" && s.Operation == "plan" {
			return fmt.Sprintf("no apply summary; plan was %d to add, %d to change, %d to destroy", s.Add, s.Change, s.Remove)
		}
		var parts []string
		switch s.Operation {
		case "plan":
			if s.Import > 0 {
				parts = append(parts, fmt.Sprintf("%d to import", s.Import))
			}
			parts = append(parts, fmt.Sprintf("%d to add", s.Add), fmt.Sprintf("%d to change", s.Change), fmt.Sprintf("%d to destroy", s.Remove))
			if s.ActionInvocation > 0 {
				parts = append(parts, fmt.Sprintf("%d %s to invoke", s.ActionInvocation, plural(s.ActionInvocation, "action", "actions")))
			}
		case "apply":
			if s.Import > 0 {
				parts = append(parts, fmt.Sprintf("%d imported", s.Import))
			}
			parts = append(parts, fmt.Sprintf("%d added", s.Add), fmt.Sprintf("%d changed", s.Change), fmt.Sprintf("%d destroyed", s.Remove))
			parts = appendInvoked(parts, s)
		case "destroy":
			parts = append(parts, fmt.Sprintf("%d destroyed", s.Remove))
			parts = appendInvoked(parts, s)
		default:
			parts = append(parts, fmt.Sprintf("%d add", s.Add), fmt.Sprintf("%d change", s.Change), fmt.Sprintf("%d destroy", s.Remove))
		}
		return strings.Join(parts, ", ")
	}
	if t := r.Tests; t != nil && t.Status != "" {
		return fmt.Sprintf("%s, %d passed, %d failed, %d errored, %d skipped", t.Status, t.Passed, t.Failed, t.Errored, t.Skipped)
	}
	if r.Providers > 0 {
		return fmt.Sprintf("%d %s installed", r.Providers, plural(r.Providers, "provider", "providers"))
	}
	return ""
}

func appendInvoked(parts []string, s *Summary) []string {
	if s.ActionInvocation > 0 {
		parts = append(parts, fmt.Sprintf("%d %s invoked", s.ActionInvocation, plural(s.ActionInvocation, "action", "actions")))
	}
	if s.ActionFail > 0 {
		parts = append(parts, fmt.Sprintf("%d %s failed", s.ActionFail, plural(s.ActionFail, "action", "actions")))
	}
	return parts
}

func writeChange(b *strings.Builder, c Change) {
	b.WriteString(symbol(c.Action))
	b.WriteByte(' ')
	b.WriteString(c.Address)
	var notes []string
	if _, known := actionSymbols[c.Action]; !known {
		notes = append(notes, "action "+c.Action)
	}
	if c.PreviousAddress != "" {
		notes = append(notes, "moved from "+c.PreviousAddress)
	}
	if c.ImportID != "" {
		notes = append(notes, "import id "+c.ImportID)
	}
	if c.Action == "remove" {
		notes = append(notes, "removed from state, not destroyed")
	}
	if c.Reason != "" {
		notes = append(notes, c.Reason)
	}
	if len(notes) > 0 {
		b.WriteString(" (")
		b.WriteString(strings.Join(notes, "; "))
		b.WriteByte(')')
	}
	b.WriteByte('\n')
}

func seconds(s float64) string {
	return strconv.FormatFloat(s, 'f', -1, 64) + "s"
}

func writeApplied(b *strings.Builder, a Applied) {
	if a.Count == 0 {
		return
	}
	fmt.Fprintf(b, "applied: %d %s", a.Count, plural(a.Count, "resource", "resources"))
	if len(a.Slowest) > 0 {
		items := make([]string, 0, len(a.Slowest))
		for _, t := range a.Slowest {
			items = append(items, t.Address+" "+seconds(t.Seconds))
		}
		b.WriteString(" (slowest: ")
		b.WriteString(strings.Join(items, ", "))
		b.WriteByte(')')
	}
	b.WriteByte('\n')
}

func writeDrift(b *strings.Builder, drift []Change, list bool) {
	if len(drift) == 0 {
		return
	}
	if !list {
		fmt.Fprintf(b, "drift: %d %s (--drift to list)\n", len(drift), plural(len(drift), "resource", "resources"))
		return
	}
	b.WriteString("drift:\n")
	for _, c := range drift {
		writeChange(b, c)
	}
}

func writeOutputs(b *strings.Builder, outputs []Output) {
	if len(outputs) == 0 {
		return
	}
	items := make([]string, 0, len(outputs))
	for _, o := range outputs {
		item := o.Name
		if o.Action != "" && o.Action != "noop" {
			item = symbol(o.Action) + " " + item
		}
		switch {
		case o.Sensitive:
			item += " (sensitive)"
		case len(o.Value) > 0 && len(bytes.TrimSpace(o.Value)) <= maxValueBytes:
			item += " = " + string(bytes.TrimSpace(o.Value))
		}
		items = append(items, item)
	}
	b.WriteString("outputs: ")
	b.WriteString(strings.Join(items, ", "))
	b.WriteByte('\n')
}

func writeTests(b *strings.Builder, t *Tests, o Options) {
	if t == nil {
		return
	}
	listed := map[string]bool{}
	for _, run := range t.Runs {
		if !o.All && !failingTestStatus[run.Status] {
			continue
		}
		listed[run.File] = true
		status := run.Status
		if status == "" {
			status = "unknown"
		}
		fmt.Fprintf(b, "%s %s %q", status, run.File, run.Run)
		if run.ElapsedMs != nil && *run.ElapsedMs > 0 {
			b.WriteString(" " + seconds(float64(*run.ElapsedMs)/1000))
		}
		b.WriteByte('\n')
		for _, d := range run.Diagnostics {
			tfdiag.Write(b, d, tfdiag.WriteOptions{Indent: "  ", NoDetail: o.NoDetail})
		}
	}
	for _, f := range t.Files {
		failing := failingTestStatus[f.Status] && !listed[f.File]
		if !failing && len(f.Diagnostics) == 0 && !o.All {
			continue
		}
		status := f.Status
		if status == "" {
			status = "unknown"
		}
		fmt.Fprintf(b, "%s %s\n", status, f.File)
		for _, d := range f.Diagnostics {
			tfdiag.Write(b, d, tfdiag.WriteOptions{Indent: "  ", NoDetail: o.NoDetail})
		}
	}
}

func writeOther(b *strings.Builder, r Report, all bool) {
	total := 0
	for _, n := range r.Other {
		total += n
	}
	if total == 0 {
		return
	}
	if all {
		b.WriteString("other:\n")
		for _, m := range r.otherMessages {
			if len(m) > maxMessageBytes {
				cut := maxMessageBytes - 3
				for cut > 0 && m[cut]&0xC0 == 0x80 {
					cut--
				}
				m = m[:cut] + "..."
			}
			b.WriteString("  ")
			b.WriteString(m)
			b.WriteByte('\n')
		}
		return
	}
	keys := make([]string, 0, len(r.Other))
	for k := range r.Other {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if r.Other[keys[i]] != r.Other[keys[j]] {
			return r.Other[keys[i]] > r.Other[keys[j]]
		}
		return keys[i] < keys[j]
	})
	items := make([]string, 0, len(keys))
	for _, k := range keys {
		items = append(items, fmt.Sprintf("%s %d", k, r.Other[k]))
	}
	fmt.Fprintf(b, "... %d other %s: %s (--all)\n", total, plural(total, "message", "messages"), strings.Join(items, ", "))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

package tfjson

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/sanitize"

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

var leftPrefixes = map[leftReason]string{
	leftCleanupFailed: "left in state after", leftSkipCleanup: "left in state after",
	leftInterrupted: "left in state after", leftPlanned: "maybe created by",
}

var leftLabels = map[leftReason]string{
	leftCleanupFailed: "cleanup failed", leftSkipCleanup: "skip_cleanup",
	leftInterrupted: "interrupted", leftPlanned: "interrupted",
}

func symbol(action string) string {
	if s, ok := actionSymbols[action]; ok {
		return s
	}
	return "?"
}

func text(r report, o options) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, c := range r.Changes {
		writeChange(&b, c)
	}
	if r.Omitted > 0 {
		fmt.Fprintf(&b, "... %d more %s (--max-resources 0)\n", r.Omitted, core.Plural(r.Omitted, "resource", "resources"))
	}
	if r.NoopHidden > 0 {
		fmt.Fprintf(&b, "... %d no-op %s (--all)\n", r.NoopHidden, core.Plural(r.NoopHidden, "change", "changes"))
	}
	writeLists(&b, r)
	tfdiag.WriteActions(&b, r.Actions)
	writeApplied(&b, r.Applied)
	if r.ActionsCompleted > 0 {
		fmt.Fprintf(&b, "actions completed: %d\n", r.ActionsCompleted)
	}
	for _, f := range r.Failures {
		b.WriteString("error: ")
		b.WriteString(f.Message)
		b.WriteByte('\n')
	}
	for _, p := range r.Policies {
		b.WriteString(policyLine(p))
		b.WriteByte('\n')
	}
	for _, d := range r.Diagnostics {
		tfdiag.Write(&b, d, tfdiag.WriteOptions{NoDetail: o.NoDetail})
	}
	if r.DiagnosticsOmitted > 0 {
		fmt.Fprintf(&b, "... %d more %s (--max-diagnostics 0)\n", r.DiagnosticsOmitted, core.Plural(r.DiagnosticsOmitted, "diagnostic", "diagnostics"))
	}
	writeDrift(&b, r, o.ListDrift)
	if len(r.Deferred) > 0 {
		b.WriteString("deferred:\n")
		for _, d := range r.Deferred {
			fmt.Fprintf(&b, "%s %s (%s)\n", symbol(d.Action), d.Address, d.Reason)
		}
		if r.DeferredOmitted > 0 {
			fmt.Fprintf(&b, "... %d more deferred %s (--max-resources 0)\n", r.DeferredOmitted, core.Plural(r.DeferredOmitted, "resource", "resources"))
		}
	}
	writeOutputs(&b, r.Outputs)
	writeTests(&b, r.Tests, o)
	writeOther(&b, r, o.All)
	return b.String()
}

func policyLine(p policyResult) string {
	line := "policy:"
	if p.Policy != "" {
		line += " " + p.Policy
	}
	line += " " + p.Result
	if p.Target != "" {
		line += " for " + p.Target
	}
	if p.Enforcement != "" {
		line += " (" + p.Enforcement + ")"
	}
	return line
}

func header(r report) string {
	line := core.ToolLabel(r.Tool, r.ToolVersion)
	if r.Operation != "" {
		line += " " + r.Operation
	}
	var segments []string
	if words := summaryWords(r); words != "" {
		segments = append(segments, words)
	}
	if n := len(r.Drift) + r.DriftOmitted; n > 0 {
		segments = append(segments, fmt.Sprintf("%d drifted", n))
	}
	if n := len(r.Deferred) + r.DeferredOmitted; n > 0 {
		segments = append(segments, fmt.Sprintf("%d deferred", n))
	}
	if diags := tfdiag.CountWords(r.Errors, r.Warnings); len(diags) > 0 {
		segments = append(segments, strings.Join(diags, ", "))
	}
	if len(segments) > 0 {
		line += ": " + strings.Join(segments, "; ")
	}
	return line
}

func summaryWords(r report) string {
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
				parts = append(parts, fmt.Sprintf("%d %s to invoke", s.ActionInvocation, core.Plural(s.ActionInvocation, "action", "actions")))
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
	if n := len(r.Lists); n > 0 {
		total := 0
		for _, l := range r.Lists {
			total += l.Total
		}
		return fmt.Sprintf("%d found in %d %s", total, n, core.Plural(n, "list block", "list blocks"))
	}
	if r.Providers > 0 {
		return fmt.Sprintf("%d %s installed", r.Providers, core.Plural(r.Providers, "provider", "providers"))
	}
	return ""
}

func appendInvoked(parts []string, s *summary) []string {
	if s.ActionInvocation > 0 {
		parts = append(parts, fmt.Sprintf("%d %s invoked", s.ActionInvocation, core.Plural(s.ActionInvocation, "action", "actions")))
	}
	if s.ActionFail > 0 {
		parts = append(parts, fmt.Sprintf("%d %s failed", s.ActionFail, core.Plural(s.ActionFail, "action", "actions")))
	}
	return parts
}

func writeChange(b *strings.Builder, c change) {
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

func writeLists(b *strings.Builder, r report) {
	found := map[string][]foundResource{}
	for _, f := range r.Found {
		found[f.Address] = append(found[f.Address], f)
	}
	for _, l := range r.Lists {
		fmt.Fprintf(b, "%s: %d found", l.Address, l.Total)
		if !l.Complete {
			b.WriteString(", incomplete")
		}
		b.WriteByte('\n')
		for _, f := range found[l.Address] {
			name := f.DisplayName
			if name == "" {
				name = "(no display name)"
			}
			b.WriteString("  " + name + "\n")
		}
	}
	if r.FoundOmitted > 0 {
		fmt.Fprintf(b, "... %d more found %s (--max-resources 0)\n", r.FoundOmitted, core.Plural(r.FoundOmitted, "resource", "resources"))
	}
}

func writeApplied(b *strings.Builder, a applied) {
	if a.Count == 0 {
		return
	}
	fmt.Fprintf(b, "applied: %d %s", a.Count, core.Plural(a.Count, "resource", "resources"))
	if len(a.Slowest) > 0 {
		items := make([]string, 0, len(a.Slowest))
		for _, t := range a.Slowest {
			items = append(items, t.Address+" "+core.Seconds(t.Seconds))
		}
		b.WriteString(" (slowest: ")
		b.WriteString(strings.Join(items, ", "))
		b.WriteByte(')')
	}
	b.WriteByte('\n')
}

func writeDrift(b *strings.Builder, r report, list bool) {
	n := len(r.Drift) + r.DriftOmitted
	if n == 0 {
		return
	}
	if !list {
		fmt.Fprintf(b, "... %d drifted %s (--drift)\n", n, core.Plural(n, "resource", "resources"))
		return
	}
	b.WriteString("drift:\n")
	for _, c := range r.Drift {
		writeChange(b, c)
	}
	if r.DriftOmitted > 0 {
		fmt.Fprintf(b, "... %d more drifted %s (--max-resources 0)\n", r.DriftOmitted, core.Plural(r.DriftOmitted, "resource", "resources"))
	}
}

func writeOutputs(b *strings.Builder, outputs []output) {
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
		default:
			if v := sanitize.Line(string(bytes.TrimSpace(o.Value))); v != "" && len(v) <= maxValueBytes {
				item += " = " + v
			}
		}
		items = append(items, item)
	}
	b.WriteString("outputs: ")
	b.WriteString(strings.Join(items, ", "))
	b.WriteByte('\n')
}

func writeTests(b *strings.Builder, t *tests, o options) {
	if t == nil {
		return
	}
	listed := map[string]bool{}
	passing := 0
	for _, run := range t.Runs {
		if !o.All && !failingTestStatus[run.Status] && len(run.Diagnostics) == 0 {
			passing++
			continue
		}
		listed[run.File] = true
		status := run.Status
		if status == "" {
			status = "unknown"
		}
		fmt.Fprintf(b, "%s %s %q", status, run.File, run.Run)
		if run.ElapsedMs != nil && *run.ElapsedMs > 0 {
			b.WriteString(" " + core.Seconds(float64(*run.ElapsedMs)/1000))
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
	for _, l := range t.Left {
		writeLeft(b, l)
	}
	if passing > 0 {
		fmt.Fprintf(b, "... %d passing test %s (--all)\n", passing, core.Plural(passing, "run", "runs"))
	}
}

func writeLeft(b *strings.Builder, l leftInState) {
	b.WriteString(leftPrefixes[l.Reason] + " " + l.File)
	if l.Run != "" {
		fmt.Fprintf(b, " %q", l.Run)
	}
	label := leftLabels[l.Reason]
	if l.State != "" {
		label += ", state " + l.State
	}
	items := make([]string, 0, len(l.Resources))
	for _, r := range l.Resources {
		item := r.Address
		if r.DeposedKey != "" {
			item += " (deposed " + r.DeposedKey + ")"
		}
		items = append(items, item)
	}
	fmt.Fprintf(b, " (%s): %s\n", label, strings.Join(items, ", "))
}

func writeOther(b *strings.Builder, r report, all bool) {
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
			m = core.Ellipsis(m, maxMessageBytes)
			b.WriteString("  ")
			b.WriteString(m)
			b.WriteByte('\n')
		}
		return
	}
	counts := core.SortedCounts(r.Other)
	items := make([]string, 0, len(counts))
	for _, c := range counts {
		items = append(items, fmt.Sprintf("%s %d", core.NoticeName(c.Name), c.Count))
	}
	fmt.Fprintf(b, "... %d other %s: %s (--all)\n", total, core.Plural(total, "message", "messages"), strings.Join(items, ", "))
}

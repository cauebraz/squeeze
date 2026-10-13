package pytest

import (
	"fmt"
	"github.com/cauebraz/squeeze/internal/core"
	"strings"
)

func Text(r Report, o Options) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	if r.Interrupted != "" {
		fmt.Fprintf(&b, "interrupted: %s\n", r.Interrupted)
	}
	if r.Stopped != "" {
		fmt.Fprintf(&b, "stopped: %s\n", r.Stopped)
	}
	for _, f := range r.Failures {
		writeFailure(&b, f, o.Full)
	}
	if r.FailuresOmitted > 0 {
		fmt.Fprintf(&b, "... %d more %s (--max-tests 0)\n", r.FailuresOmitted, core.Plural(r.FailuresOmitted, "failure or error", "failures and errors"))
	}
	for _, w := range r.Warnings {
		fmt.Fprintf(&b, "warning %s", w.Message)
		if w.Count > 1 {
			fmt.Fprintf(&b, " (x%d)", w.Count)
		}
		b.WriteByte('\n')
	}
	if r.WarningsOmitted > 0 {
		fmt.Fprintf(&b, "... %d more %s (--max-warnings 0)\n", r.WarningsOmitted, core.Plural(r.WarningsOmitted, "warning", "warnings"))
	}
	if len(r.Notes) > 0 {
		b.WriteString("output outside the report sections:\n")
		if r.NotesOmitted > 0 {
			fmt.Fprintf(&b, "  ... %d earlier %s (--max-output 0)\n", r.NotesOmitted, core.Plural(r.NotesOmitted, "line", "lines"))
		}
		for _, n := range r.Notes {
			b.WriteString("  " + n + "\n")
		}
	}
	if o.All {
		for _, l := range r.Outcomes {
			b.WriteString(l + "\n")
		}
		for _, title := range r.sectionOrder {
			fmt.Fprintf(&b, "section %s:\n", title)
			for _, l := range r.sectionLines[title] {
				b.WriteString("  " + l + "\n")
			}
		}
	} else {
		writeCounted(&b, r)
	}
	return b.String()
}

func header(r Report) string {
	tool := r.Tool
	if r.Version != "" {
		tool += " " + r.Version
	}
	if !r.Complete {
		return tool + ": no summary line"
	}
	return tool + ": " + r.Summary.Line
}

func writeFailure(b *strings.Builder, f Failure, full bool) {
	word := "FAILED"
	if f.Kind == "error" {
		word = "ERROR"
	}
	name := f.Test
	if name == "" {
		name = "(entry without a node id)"
	}
	fmt.Fprintf(b, "%s %s", word, name)
	if f.Subtest != "" {
		b.WriteString(" " + f.Subtest)
	}
	if f.When != "" {
		fmt.Fprintf(b, " (%s)", f.When)
	}
	if f.Untracked && f.Message != "" {
		b.WriteString(" - " + f.Message)
	}
	b.WriteByte('\n')
	if f.Omitted > 0 {
		fmt.Fprintf(b, "  ... %d earlier %s (--max-output 0)\n", f.Omitted, core.Plural(f.Omitted, "line", "lines"))
	}
	for _, l := range f.Lines {
		b.WriteString("  " + l + "\n")
	}
	var hidden []string
	if f.Hidden > 0 {
		hidden = append(hidden, fmt.Sprintf("%d traceback %s", f.Hidden, core.Plural(f.Hidden, "line", "lines")))
	}
	if len(hidden) > 0 {
		fmt.Fprintf(b, "  ... %s (--full)\n", strings.Join(hidden, "; "))
	}
	if !full {
		names := make([]string, 0, len(f.Captured))
		count := 0
		for _, c := range f.Captured {
			names = append(names, c.Name)
			count += len(c.Lines) + c.Omitted
		}
		if len(f.Captured) > 0 {
			fmt.Fprintf(b, "  ... %d captured %s in %s (--full)\n", count, core.Plural(count, "line", "lines"), strings.Join(names, " and "))
		}
		return
	}
	for _, c := range f.Captured {
		fmt.Fprintf(b, "  captured %s:\n", c.Name)
		if c.Omitted > 0 {
			fmt.Fprintf(b, "    ... %d earlier %s (--max-output 0)\n", c.Omitted, core.Plural(c.Omitted, "line", "lines"))
		}
		for _, l := range c.Lines {
			b.WriteString("    " + l + "\n")
		}
	}
}

func writeCounted(b *strings.Builder, r Report) {
	if n := len(r.Outcomes); n > 0 {
		fmt.Fprintf(b, "... %d short summary %s for passed, skipped, xfailed or xpassed tests (--all)\n", n, core.Plural(n, "line", "lines"))
	}
	for _, title := range r.sectionOrder {
		n := r.Sections[title]
		fmt.Fprintf(b, "... %d %s in section %q (--all)\n", n, core.Plural(n, "line", "lines"), title)
	}
}

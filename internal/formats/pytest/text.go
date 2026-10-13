package pytest

import (
	"fmt"
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
		writeFailure(&b, f)
	}
	if r.FailuresOmitted > 0 {
		fmt.Fprintf(&b, "... %d more failures and errors (--max-tests 0)\n", r.FailuresOmitted)
	}
	for _, w := range r.Warnings {
		fmt.Fprintf(&b, "warning %s", w.Message)
		if w.Count > 1 {
			fmt.Fprintf(&b, " (x%d)", w.Count)
		}
		b.WriteByte('\n')
	}
	if r.WarningsOmitted > 0 {
		fmt.Fprintf(&b, "... %d more %s (--max-warnings 0)\n", r.WarningsOmitted, plural(r.WarningsOmitted, "warning", "warnings"))
	}
	if len(r.Notes) > 0 {
		b.WriteString("output outside the report sections:\n")
		for _, n := range r.Notes {
			b.WriteString("  " + n + "\n")
		}
		if r.NotesOmitted > 0 {
			fmt.Fprintf(&b, "  ... %d more %s (--max-output 0)\n", r.NotesOmitted, plural(r.NotesOmitted, "line", "lines"))
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

func writeFailure(b *strings.Builder, f Failure) {
	word := "FAILED"
	if f.Kind == "error" {
		word = "ERROR"
	}
	fmt.Fprintf(b, "%s %s", word, f.Test)
	if f.When != "" {
		fmt.Fprintf(b, " (%s)", f.When)
	}
	if f.Untracked && f.Message != "" {
		b.WriteString(" - " + f.Message)
	}
	b.WriteByte('\n')
	if f.Omitted > 0 {
		fmt.Fprintf(b, "  ... %d earlier %s (--max-output 0)\n", f.Omitted, plural(f.Omitted, "line", "lines"))
	}
	for _, l := range f.Lines {
		b.WriteString("  " + l + "\n")
	}
	var hidden []string
	if f.Hidden > 0 {
		hidden = append(hidden, fmt.Sprintf("%d traceback %s", f.Hidden, plural(f.Hidden, "line", "lines")))
	}
	if len(f.Captured) > 0 {
		hidden = append(hidden, "captured "+strings.Join(f.Captured, ", "))
	}
	if len(hidden) > 0 {
		fmt.Fprintf(b, "  ... %s (--full)\n", strings.Join(hidden, "; "))
	}
}

func writeCounted(b *strings.Builder, r Report) {
	var items []string
	if n := len(r.Outcomes); n > 0 {
		items = append(items, fmt.Sprintf("%d short summary %s for passed, skipped, xfailed or xpassed tests", n, plural(n, "line", "lines")))
	}
	for _, title := range r.sectionOrder {
		n := r.Sections[title]
		items = append(items, fmt.Sprintf("section %q (%d %s)", title, n, plural(n, "line", "lines")))
	}
	if len(items) > 0 {
		fmt.Fprintf(b, "... not shown: %s (--all)\n", strings.Join(items, ", "))
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

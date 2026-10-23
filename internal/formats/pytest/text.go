package pytest

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

func render(r report, o options) string {
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
	if r.TestsOmitted > 0 {
		fmt.Fprintf(&b, "... %d more %s (--max-tests 0)\n", r.TestsOmitted, core.Plural(r.TestsOmitted, "failed or errored test", "failed or errored tests"))
	}
	for _, w := range r.WarningMessages {
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
		core.Block{Lines: r.Notes, Omitted: r.NotesOmitted}.WriteTail(&b, "  ", "--max-output")
	}
	if o.All {
		for _, l := range r.Outcomes {
			b.WriteString(l + "\n")
		}
		for _, sec := range r.OtherSectionLines {
			fmt.Fprintf(&b, "section %s:\n", sec.Name)
			for _, l := range sec.Lines {
				b.WriteString("  " + l + "\n")
			}
		}
	} else {
		writeCounted(&b, r)
	}
	return b.String()
}

func header(r report) string {
	tool := core.ToolLabel(r.Tool, r.ToolVersion)
	if !r.SummaryFound {
		return tool + ": no summary line"
	}
	return tool + ": " + r.Summary.Line
}

func writeFailure(b *strings.Builder, f failure, full bool) {
	name := f.Test
	if name == "" {
		name = "(entry without a node id)"
	}
	fmt.Fprintf(b, "%s %s", f.Kind.word(), name)
	if f.Subtest != "" {
		b.WriteString(" " + f.Subtest)
	}
	if f.When != "" {
		fmt.Fprintf(b, " (%s)", f.When)
	}
	if f.FromSummaryOnly && f.Message != "" {
		b.WriteString(" - " + f.Message)
	}
	b.WriteByte('\n')
	f.WriteTail(b, "  ", "--max-output")
	if f.Hidden > 0 {
		fmt.Fprintf(b, "  ... %d traceback %s (--full)\n", f.Hidden, core.Plural(f.Hidden, "line", "lines"))
	}
	writeCaptured(b, f.Captured, full)
	if n := len(f.SameLocationTests); n > 0 {
		fmt.Fprintf(b, "  ... %d more %s from %s (--no-fold)\n", n, core.Plural(n, failureWord(f), failureWord(f)+"s"), f.Location)
	}
}

func writeCaptured(b *strings.Builder, captured []capturedSection, full bool) {
	if full {
		for _, c := range captured {
			fmt.Fprintf(b, "  captured %s:\n", c.Name)
			c.WriteTail(b, "    ", "--max-output")
		}
		return
	}
	if len(captured) == 0 {
		return
	}
	names := make([]string, 0, len(captured))
	count := 0
	for _, c := range captured {
		names = append(names, c.Name)
		count += len(c.Lines) + c.Omitted
	}
	fmt.Fprintf(b, "  ... %d captured %s in %s (--full)\n", count, core.Plural(count, "line", "lines"), strings.Join(names, " and "))
}

func failureWord(f failure) string {
	switch f.Kind {
	case kindError:
		return "error"
	case kindFailed:
	}
	return "failure"
}

func writeCounted(b *strings.Builder, r report) {
	if n := len(r.Outcomes); n > 0 {
		fmt.Fprintf(b, "... %d short summary %s for passed, skipped, xfailed or xpassed tests (--all)\n", n, core.Plural(n, "line", "lines"))
	}
	for _, sec := range r.OtherSectionLines {
		n := len(sec.Lines)
		fmt.Fprintf(b, "... %d %s in section %q (--all)\n", n, core.Plural(n, "line", "lines"), sec.Name)
	}
}

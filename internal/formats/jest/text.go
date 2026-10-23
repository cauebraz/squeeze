package jest

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

func render(r report, o options) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	if r.RunError != nil {
		b.WriteString("run error\n")
		writeLines(&b, *r.RunError, "    ")
	}
	errs := map[string]output{}
	for _, e := range r.FileErrors {
		errs[e.File] = e.Output
	}
	byFile := map[string][]failure{}
	var order []string
	for _, f := range r.Failures {
		if _, ok := byFile[f.File]; !ok {
			order = append(order, f.File)
		}
		byFile[f.File] = append(byFile[f.File], f)
	}
	for _, e := range r.FileErrors {
		if _, ok := byFile[e.File]; !ok {
			order = append(order, e.File)
		}
	}
	slices.Sort(order)
	for _, file := range order {
		fails := byFile[file]
		if out, ok := errs[file]; ok {
			fmt.Fprintf(&b, "%s: failed to run\n", file)
			writeLines(&b, out, "    ")
		} else {
			fmt.Fprintf(&b, "%s\n", file)
		}
		for _, f := range fails {
			fmt.Fprintf(&b, "  fail %s\n", f.Test)
			writeLines(&b, f.Output, "    ")
		}
	}
	if r.TestsOmitted > 0 {
		fmt.Fprintf(&b, "... %d more failed %s (--max-tests 0)\n", r.TestsOmitted, core.Plural(r.TestsOmitted, "test", "tests"))
	}
	for _, h := range r.OpenHandles {
		fmt.Fprintf(&b, "open handle: %s\n", handleLabel(h))
		writeLines(&b, h.Output, "    ")
	}
	if len(r.Trailing.Lines) > 0 {
		b.WriteString("output after the JSON:\n")
		writeLines(&b, r.Trailing, "    ")
	}
	if o.All {
		for _, t := range r.NotRun {
			fmt.Fprintf(&b, "%s %s: %s\n", t.Status, t.File, t.Test)
		}
		for _, f := range r.PassingFiles {
			fmt.Fprintf(&b, "pass %s\n", f)
		}
	} else {
		skipped, todo := 0, 0
		for _, t := range r.NotRun {
			if t.Status == testTodo {
				todo++
			} else {
				skipped++
			}
		}
		if items := core.JoinCounts(
			core.Part{N: skipped, One: "skipped test", Many: "skipped tests"},
			core.Part{N: todo, One: "todo test", Many: "todo tests"},
			core.Part{N: len(r.PassingFiles), One: "passing file", Many: "passing files"},
		); items != "" {
			fmt.Fprintf(&b, "... %s (--all)\n", items)
		}
	}
	if notice := core.UnknownNotice(r.UnknownStatus, "test", "tests", "status"); notice != "" {
		b.WriteString(notice + "\n")
	}
	return b.String()
}

func handleLabel(h handle) string {
	if h.Message != "" {
		return h.Message
	}
	return h.Name
}

func header(r report) string {
	t := r.Tests
	line := fmt.Sprintf("%s: %s; %d %s: %d passed, %d failed", r.Tool, r.Status, t.Total, core.Plural(t.Total, "test", "tests"), t.Passed, t.Failed)
	if more := core.JoinCounts(core.Part{N: t.Skipped, One: "skipped"}, core.Part{N: t.Todo, One: "todo"}); more != "" {
		line += ", " + more
	}
	f := r.Files
	if files := core.JoinCounts(core.Part{N: f.Passed, One: "passed"}, core.Part{N: f.Failed, One: "failed"}, core.Part{N: f.FailedToRun, One: "failed to run"}, core.Part{N: f.Skipped, One: "skipped"}); files != "" {
		line += fmt.Sprintf("; %d %s: %s", f.Total, core.Plural(f.Total, "file", "files"), files)
	}
	s := r.Snapshots
	if snaps := core.JoinCounts(core.Part{N: s.Failed, One: "failed"}, core.Part{N: s.Obsolete, One: "obsolete"}, core.Part{N: s.FilesRemoved, One: "removed file", Many: "removed files"}, core.Part{N: s.Written, One: "written"}, core.Part{N: s.Updated, One: "updated"}); snaps != "" {
		line += fmt.Sprintf("; %d %s: %s", s.Total, core.Plural(s.Total, "snapshot", "snapshots"), snaps)
	}
	if r.Interrupted {
		line += "; interrupted"
	}
	if n := len(r.OpenHandles); n > 0 {
		line += fmt.Sprintf("; %d open %s", n, core.Plural(n, "handle", "handles"))
	}
	return line
}

func writeLines(b *strings.Builder, l output, indent string) {
	l.Write(b, indent, "--max-output")
	if l.Hidden > 0 {
		fmt.Fprintf(b, "%s... %d node_modules, node internal and code frame %s (--full)\n", indent, l.Hidden, core.Plural(l.Hidden, "line", "lines"))
	}
}

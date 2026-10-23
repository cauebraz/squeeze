package jest

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

type part struct {
	n    int
	what string
	many string
}

func Text(r Report, o Options) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	if r.RunError != nil {
		b.WriteString("run error\n")
		writeLines(&b, *r.RunError, "    ")
	}
	errs := map[string]Lines{}
	for _, e := range r.FileErrors {
		errs[e.File] = e.Output
	}
	byFile := map[string][]Failure{}
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
	sort.Strings(order)
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
	if r.FailuresOmitted > 0 {
		fmt.Fprintf(&b, "... %d more failed %s (--max-tests 0)\n", r.FailuresOmitted, core.Plural(r.FailuresOmitted, "test", "tests"))
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
		for _, f := range r.Passing {
			fmt.Fprintf(&b, "pass %s\n", f)
		}
	} else {
		skipped, todo := 0, 0
		for _, t := range r.NotRun {
			if t.Status == "todo" {
				todo++
			} else {
				skipped++
			}
		}
		var items []string
		for _, c := range []struct {
			n          int
			what, many string
		}{{skipped, "skipped test", "skipped tests"}, {todo, "todo test", "todo tests"}, {len(r.Passing), "passing file", "passing files"}} {
			if c.n > 0 {
				items = append(items, fmt.Sprintf("%d %s", c.n, core.Plural(c.n, c.what, c.many)))
			}
		}
		if len(items) > 0 {
			fmt.Fprintf(&b, "... %s (--all)\n", strings.Join(items, ", "))
		}
	}
	writeUnknown(&b, r.UnknownStatus)
	return b.String()
}

func handleLabel(h Handle) string {
	if h.Message != "" {
		return h.Message
	}
	return h.Name
}

func header(r Report) string {
	t := r.Tests
	tests := []part{{t.Passed, "passed", ""}, {t.Failed, "failed", ""}}
	if t.Skipped > 0 {
		tests = append(tests, part{t.Skipped, "skipped", ""})
	}
	if t.Todo > 0 {
		tests = append(tests, part{t.Todo, "todo", ""})
	}
	line := fmt.Sprintf("%s: %s; %d %s: %s", r.Tool, r.Status, t.Total, core.Plural(t.Total, "test", "tests"), join(tests))
	f := r.Files
	if files := nonZero([]part{{f.Passed, "passed", ""}, {f.Failed, "failed", ""}, {f.FailedToRun, "failed to run", ""}, {f.Skipped, "skipped", ""}}); len(files) > 0 {
		line += fmt.Sprintf("; %d %s: %s", f.Total, core.Plural(f.Total, "file", "files"), join(files))
	}
	s := r.Snapshots
	if snaps := nonZero([]part{{s.Failed, "failed", ""}, {s.Obsolete, "obsolete", ""}, {s.Removed, "removed file", "removed files"}, {s.Written, "written", ""}, {s.Updated, "updated", ""}}); len(snaps) > 0 {
		line += fmt.Sprintf("; %d %s: %s", s.Total, core.Plural(s.Total, "snapshot", "snapshots"), join(snaps))
	}
	if r.Interrupted {
		line += "; interrupted"
	}
	if n := len(r.OpenHandles); n > 0 {
		line += fmt.Sprintf("; %d open %s", n, core.Plural(n, "handle", "handles"))
	}
	return line
}

func nonZero(parts []part) []part {
	var out []part
	for _, p := range parts {
		if p.n > 0 {
			out = append(out, p)
		}
	}
	return out
}

func join(parts []part) string {
	items := make([]string, 0, len(parts))
	for _, p := range parts {
		what := p.what
		if p.many != "" {
			what = core.Plural(p.n, p.what, p.many)
		}
		items = append(items, fmt.Sprintf("%d %s", p.n, what))
	}
	return strings.Join(items, ", ")
}

func writeLines(b *strings.Builder, l Lines, indent string) {
	for _, line := range l.Lines {
		b.WriteString(indent)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if l.Omitted > 0 {
		fmt.Fprintf(b, "%s... %d more %s (--max-output 0)\n", indent, l.Omitted, core.Plural(l.Omitted, "line", "lines"))
	}
	if l.Hidden > 0 {
		fmt.Fprintf(b, "%s... %d node_modules, node internal and code frame %s (--full)\n", indent, l.Hidden, core.Plural(l.Hidden, "line", "lines"))
	}
}

func writeUnknown(b *strings.Builder, unknown map[string]int) {
	if len(unknown) == 0 {
		return
	}
	keys := make([]string, 0, len(unknown))
	total := 0
	for k, n := range unknown {
		keys = append(keys, k)
		total += n
	}
	sort.Strings(keys)
	items := make([]string, 0, len(keys))
	for _, k := range keys {
		name := core.NoticeName(k)
		if name == "" {
			name = "empty"
		}
		items = append(items, fmt.Sprintf("%s %d", name, unknown[k]))
	}
	fmt.Fprintf(b, "... %d %s with an unknown status: %s (--raw-dir)\n", total, core.Plural(total, "test", "tests"), strings.Join(items, ", "))
}

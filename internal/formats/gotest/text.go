package gotest

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func Text(r Report, o Options) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, f := range r.BuildFailures {
		fmt.Fprintf(&b, "build failed: %s\n", f.Package)
		writeOutput(&b, f.Output)
	}
	for _, f := range r.Failures {
		fmt.Fprintf(&b, "fail %s %s", f.Package, f.Test)
		if f.Elapsed > 0 {
			b.WriteString(" " + seconds(f.Elapsed))
		}
		if f.Unfinished {
			b.WriteString(" (did not finish)")
		}
		b.WriteByte('\n')
		for _, k := range sortedKeys(f.Attrs) {
			fmt.Fprintf(&b, "  attr %s=%s\n", k, f.Attrs[k])
		}
		if f.Artifacts != "" {
			fmt.Fprintf(&b, "  artifacts: %s\n", f.Artifacts)
		}
		writeOutput(&b, f.Output)
	}
	if r.FailuresOmitted > 0 {
		fmt.Fprintf(&b, "... %d more failed %s (--max-tests 0)\n", r.FailuresOmitted, plural(r.FailuresOmitted, "test", "tests"))
	}
	for _, p := range r.PackageFailures {
		fmt.Fprintf(&b, "fail %s", p.Package)
		if p.Unfinished {
			b.WriteString(" (did not finish)")
		}
		b.WriteByte('\n')
		writeOutput(&b, p.Output)
	}
	for _, bm := range r.Benchmarks {
		fmt.Fprintf(&b, "bench %s %s\n", bm.Package, bm.Result)
	}
	if len(r.Text.Lines) > 0 {
		b.WriteString("output outside the test events:\n")
		writeOutput(&b, r.Text)
	}
	if o.All {
		for _, s := range r.Skipped {
			fmt.Fprintf(&b, "skip %s %s\n", s.Package, s.Test)
		}
		for _, p := range r.Passing {
			fmt.Fprintf(&b, "ok %s", p.Package)
			if p.Elapsed > 0 {
				b.WriteString(" " + seconds(p.Elapsed))
			}
			b.WriteByte('\n')
		}
		for _, p := range r.NoTests {
			fmt.Fprintf(&b, "no tests %s\n", p)
		}
	}
	writeOther(&b, r.Other)
	return b.String()
}

func header(r Report) string {
	t, p := r.Tests, r.Packages
	total := t.Passed + t.Failed + t.Skipped
	line := fmt.Sprintf("go test: %s; %d %s: %d passed, %d failed, %d skipped", r.Status, total, plural(total, "test", "tests"), t.Passed, t.Failed, t.Skipped)
	pkgs := p.OK + p.Failed + p.BuildFailed + p.NoTests
	if pkgs == 0 {
		return line
	}
	var parts []string
	for _, c := range []struct {
		n    int
		what string
	}{{p.OK, "ok"}, {p.Failed, "failed"}, {p.BuildFailed, "build failed"}, {p.NoTests, "without tests"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, c.what))
		}
	}
	return fmt.Sprintf("%s; %d %s: %s", line, pkgs, plural(pkgs, "package", "packages"), strings.Join(parts, ", "))
}

func writeOutput(b *strings.Builder, o Output) {
	for _, l := range o.Lines {
		b.WriteString("  ")
		b.WriteString(l)
		b.WriteByte('\n')
	}
	if o.Omitted > 0 {
		fmt.Fprintf(b, "  ... %d more %s (--max-output 0)\n", o.Omitted, plural(o.Omitted, "line", "lines"))
	}
	if o.Logs > 0 {
		fmt.Fprintf(b, "  ... %d log %s (--logs)\n", o.Logs, plural(o.Logs, "line", "lines"))
	}
	if o.Stack > 0 {
		fmt.Fprintf(b, "  ... %d runtime and testing stack %s (--logs)\n", o.Stack, plural(o.Stack, "line", "lines"))
	}
}

func writeOther(b *strings.Builder, other map[string]int) {
	if len(other) == 0 {
		return
	}
	keys := make([]string, 0, len(other))
	total := 0
	for k, n := range other {
		keys = append(keys, k)
		total += n
	}
	sort.Strings(keys)
	items := make([]string, 0, len(keys))
	for _, k := range keys {
		items = append(items, fmt.Sprintf("%s %d", k, other[k]))
	}
	fmt.Fprintf(b, "... %d %s with an unknown action: %s\n", total, plural(total, "event", "events"), strings.Join(items, ", "))
}

func seconds(s float64) string {
	return strconv.FormatFloat(s, 'f', -1, 64) + "s"
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

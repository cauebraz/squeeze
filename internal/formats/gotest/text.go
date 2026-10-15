package gotest

import (
	"fmt"
	"github.com/cauebraz/squeeze/internal/core"
	"sort"
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
		if f.Runs > 1 {
			fmt.Fprintf(&b, " (run %d of %d)", f.Run, f.Runs)
		}
		if f.Elapsed > 0 {
			b.WriteString(" " + core.Seconds(f.Elapsed))
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
		fmt.Fprintf(&b, "... %d more failed %s (--max-tests 0)\n", r.FailuresOmitted, core.Plural(r.FailuresOmitted, "test", "tests"))
	}
	for _, p := range r.PackageFailures {
		fmt.Fprintf(&b, "fail %s", p.Package)
		if p.Unfinished {
			b.WriteString(" (did not finish)")
		}
		b.WriteByte('\n')
		writeOutput(&b, p.Output)
	}
	hiddenSeeds := 0
	for _, s := range r.Shuffle {
		if !s.Failed && !o.All {
			hiddenSeeds++
			continue
		}
		fmt.Fprintf(&b, "shuffle %s: -test.shuffle %s\n", s.Package, s.Seed)
	}
	for _, bm := range r.Benchmarks {
		fmt.Fprintf(&b, "bench %s %s\n", bm.Package, bm.Result)
	}
	for _, w := range r.Warnings {
		fmt.Fprintf(&b, "warning %s\n", w)
	}
	for _, c := range r.Coverage {
		fmt.Fprintf(&b, "coverage %s: %s\n", c.Package, c.Coverage)
	}
	if r.CoverageOmitted > 0 {
		fmt.Fprintf(&b, "... %d more %s with coverage (--all)\n", r.CoverageOmitted, core.Plural(r.CoverageOmitted, "package", "packages"))
	}
	if len(r.Text.Lines) > 0 {
		b.WriteString("output outside the test events:\n")
		writeOutput(&b, r.Text)
	}
	if !o.All {
		var items []string
		for _, c := range []struct {
			n          int
			what, many string
		}{{len(r.Skipped), "skipped test", "skipped tests"}, {len(r.Passing), "passing package", "passing packages"}, {len(r.NoTests), "package without tests", "packages without tests"}, {hiddenSeeds, "shuffle seed", "shuffle seeds"}} {
			if c.n > 0 {
				items = append(items, fmt.Sprintf("%d %s", c.n, core.Plural(c.n, c.what, c.many)))
			}
		}
		if len(items) > 0 {
			fmt.Fprintf(&b, "... %s (--all)\n", strings.Join(items, ", "))
		}
	}
	if o.All {
		for _, s := range r.Skipped {
			fmt.Fprintf(&b, "skip %s %s\n", s.Package, s.Test)
		}
		for _, p := range r.Passing {
			fmt.Fprintf(&b, "ok %s", p.Package)
			if p.Elapsed > 0 {
				b.WriteString(" " + core.Seconds(p.Elapsed))
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
	line := fmt.Sprintf("go test: %s; %d %s: %d passed, %d failed, %d skipped", r.Status, total, core.Plural(total, "test", "tests"), t.Passed, t.Failed, t.Skipped)
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
	return fmt.Sprintf("%s; %d %s: %s", line, pkgs, core.Plural(pkgs, "package", "packages"), strings.Join(parts, ", "))
}

func writeOutput(b *strings.Builder, o Output) {
	for _, l := range o.Lines {
		b.WriteString("  ")
		b.WriteString(l)
		b.WriteByte('\n')
	}
	if o.Omitted > 0 {
		fmt.Fprintf(b, "  ... %d more %s (--max-output 0)\n", o.Omitted, core.Plural(o.Omitted, "line", "lines"))
	}
	if o.Logs > 0 {
		fmt.Fprintf(b, "  ... %d log %s (--full)\n", o.Logs, core.Plural(o.Logs, "line", "lines"))
	}
	if o.Stack > 0 {
		fmt.Fprintf(b, "  ... %d runtime and testing stack %s (--full)\n", o.Stack, core.Plural(o.Stack, "line", "lines"))
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
	fmt.Fprintf(b, "... %d %s with an unknown action: %s (--raw-dir)\n", total, core.Plural(total, "event", "events"), strings.Join(items, ", "))
}

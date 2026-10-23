package gotest

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

func render(r report, o options) string {
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
	if r.TestsOmitted > 0 {
		fmt.Fprintf(&b, "... %d more failed %s (--max-tests 0)\n", r.TestsOmitted, core.Plural(r.TestsOmitted, "test", "tests"))
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
	for _, w := range r.WarningMessages {
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
		passingNotice := len(r.PassingPackages)
		if unlisted(r) {
			passingNotice = 0
		}
		if items := core.JoinCounts(
			core.Part{N: len(r.Skipped), One: "skipped test", Many: "skipped tests"},
			core.Part{N: passingNotice, One: "passing package", Many: "passing packages"},
			core.Part{N: len(r.PackagesWithoutTests), One: "package without tests", Many: "packages without tests"},
			core.Part{N: hiddenSeeds, One: "shuffle seed", Many: "shuffle seeds"},
		); items != "" {
			fmt.Fprintf(&b, "... %s (--all)\n", items)
		}
	}
	if o.All {
		for _, s := range r.Skipped {
			fmt.Fprintf(&b, "skip %s %s\n", s.Package, s.Test)
		}
		for _, p := range r.PassingPackages {
			fmt.Fprintf(&b, "ok %s", p.Package)
			if p.Elapsed > 0 {
				b.WriteString(" " + core.Seconds(p.Elapsed))
			}
			b.WriteByte('\n')
		}
		for _, p := range r.PackagesWithoutTests {
			fmt.Fprintf(&b, "no tests %s\n", p)
		}
	}
	if notice := core.UnknownNotice(r.Other, "event", "events", "action"); notice != "" {
		b.WriteString(notice + "\n")
	}
	return b.String()
}

func header(r report) string {
	t, p := r.Tests, r.Packages
	total := t.Passed + t.Failed + t.Skipped
	head := fmt.Sprintf("go test: %s; %d %s: %d passed, %d failed, %d skipped", r.Status, total, core.Plural(total, "test", "tests"), t.Passed, t.Failed, t.Skipped)
	pkgs := p.OK + p.Failed + p.BuildFailed + p.NoTests
	if pkgs == 0 {
		return head
	}
	suffix := ""
	if unlisted(r) {
		head = "go test: " + string(r.Status)
		suffix = " (tests not listed without -v or -json)"
	}
	parts := core.JoinCounts(core.Part{N: p.OK, One: "ok"}, core.Part{N: p.Failed, One: "failed"}, core.Part{N: p.BuildFailed, One: "build failed"}, core.Part{N: p.NoTests, One: "without tests"})
	return fmt.Sprintf("%s; %d %s: %s%s", head, pkgs, core.Plural(pkgs, "package", "packages"), parts, suffix)
}

func unlisted(r report) bool {
	return r.testsUnlisted && r.Tests.Passed+r.Tests.Failed+r.Tests.Skipped == 0 && r.Packages.OK+r.Packages.Failed > 0
}

func writeOutput(b *strings.Builder, o output) {
	o.Write(b, "  ", "--max-output")
	if o.LogsHidden > 0 {
		fmt.Fprintf(b, "  ... %d log %s (--full)\n", o.LogsHidden, core.Plural(o.LogsHidden, "line", "lines"))
	}
	if o.StackHidden > 0 {
		fmt.Fprintf(b, "  ... %d runtime and testing stack %s (--full)\n", o.StackHidden, core.Plural(o.StackHidden, "line", "lines"))
	}
}

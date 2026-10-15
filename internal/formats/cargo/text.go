package cargo

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const (
	maxCodesInNotice = 5
	maxCodeBytes     = 120
	maxLineBytes     = 200
)

func Text(r Report, o Options) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, d := range r.Diagnostics {
		pd := d.Diagnostic
		pd.Code = core.Ellipsis(pd.Code, maxCodeBytes)
		if d.Fixable {
			if pd.Address != "" {
				pd.Address += ", "
			}
			pd.Address += "fixable"
		}
		tfdiag.Write(&b, pd, tfdiag.WriteOptions{})
	}
	if r.DiagnosticsOmitted > 0 {
		fmt.Fprintf(&b, "... %d more %s, %s (--max-diagnostics 0)\n", r.DiagnosticsOmitted, core.Plural(r.DiagnosticsOmitted, "diagnostic", "diagnostics"), core.TopCounts(r.OmittedByCode, maxCodesInNotice, "code", "codes"))
	}
	for i, t := range r.Targets {
		if !failedTarget(t) {
			continue
		}
		fmt.Fprintf(&b, "failed target %s: %s", t.Name, targetCounts(t))
		if t.Rerun != "" {
			fmt.Fprintf(&b, "; rerun: cargo test %s", t.Rerun)
		}
		b.WriteByte('\n')
		writeLines(&b, "  ", t.Cause, o.MaxOutput)
		for _, f := range r.Failures {
			if f.index == i {
				writeFailure(&b, f, o)
			}
		}
	}
	if r.FailuresOmitted > 0 {
		fmt.Fprintf(&b, "... %d more failed %s (--max-failures 0)\n", r.FailuresOmitted, core.Plural(r.FailuresOmitted, "test", "tests"))
	}
	return b.String()
}

func targetCounts(t Target) string {
	if t.Result == "" {
		return fmt.Sprintf("did not finish after %d passed, %d failed", t.Passed, t.Failed)
	}
	return fmt.Sprintf("%d passed, %d failed, %d ignored", t.Passed, t.Failed, t.Ignored)
}

func writeFailure(b *strings.Builder, f Failure, o Options) {
	fmt.Fprintf(b, "  fail %s", f.Test)
	if f.File != "" {
		fmt.Fprintf(b, " (%s:%d:%d)", f.File, f.Line, f.Column)
	}
	b.WriteByte('\n')
	writeLines(b, "    ", append(append([]string(nil), f.Message...), f.Output...), o.MaxOutput)
}

func writeLines(b *strings.Builder, indent string, lines []string, limit int) {
	kept, omitted := core.Limit(lines, limit)
	for _, l := range kept {
		b.WriteString(indent)
		b.WriteString(core.Ellipsis(strings.TrimRight(core.Line(l), " "), maxLineBytes))
		b.WriteByte('\n')
	}
	if omitted > 0 {
		fmt.Fprintf(b, "%s... %d more %s (--max-output 0)\n", indent, omitted, core.Plural(omitted, "line", "lines"))
	}
}

func header(r Report) string {
	parts := []string{"cargo: " + r.Status}
	if r.Build != "" {
		build := "build " + r.Build
		if len(r.FailedUnits) > 0 {
			build += ": " + strings.Join(r.FailedUnits, ", ")
		}
		parts = append(parts, build)
	}
	diags := fmt.Sprintf("%d %s, %d %s", r.Errors, core.Plural(r.Errors, "error", "errors"), r.Warnings, core.Plural(r.Warnings, "warning", "warnings"))
	if r.Fixable > 0 {
		diags += fmt.Sprintf(", %d fixable", r.Fixable)
	}
	parts = append(parts, diags)
	if n := len(r.Targets); n > 0 {
		total := r.Tests.Passed + r.Tests.Failed + r.Tests.Ignored + r.Tests.Measured
		tests := fmt.Sprintf("%d %s: %d passed, %d failed, %d ignored", total, core.Plural(total, "test", "tests"), r.Tests.Passed, r.Tests.Failed, r.Tests.Ignored)
		if r.Tests.Measured > 0 {
			tests += fmt.Sprintf(", %d measured", r.Tests.Measured)
		}
		if r.Tests.FilteredOut > 0 {
			tests += fmt.Sprintf(", %d filtered out", r.Tests.FilteredOut)
		}
		failed := 0
		for _, t := range r.Targets {
			if failedTarget(t) {
				failed++
			}
		}
		tests += fmt.Sprintf(" in %d %s", n, core.Plural(n, "target", "targets"))
		if failed > 0 {
			tests += fmt.Sprintf(", %d failed", failed)
		}
		parts = append(parts, tests)
	}
	return strings.Join(parts, "; ")
}

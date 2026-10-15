package golangci

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

func Text(r Report) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, d := range r.Run {
		tfdiag.Write(&b, d, tfdiag.WriteOptions{})
	}
	for _, p := range r.Issues {
		d := p.Diagnostic
		if p.Fixable {
			if d.Address != "" {
				d.Address += ", "
			}
			d.Address += "fixable"
		}
		tfdiag.Write(&b, d, tfdiag.WriteOptions{})
	}
	if r.IssuesOmitted > 0 {
		fmt.Fprintf(&b, "... %d more %s, %s (--max-issues 0)\n", r.IssuesOmitted, core.Plural(r.IssuesOmitted, "issue", "issues"), core.TopCounts(r.OmittedByLinter, maxLintersInNotice, "linter", "linters"))
	}
	return b.String()
}

func header(r Report) string {
	line := fmt.Sprintf("golangci-lint: %d %s", r.Total, core.Plural(r.Total, "issue", "issues"))
	if r.Total > 0 {
		var parts []string
		if r.Errors > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", r.Errors, core.Plural(r.Errors, "error", "errors")))
		}
		if r.Warnings > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", r.Warnings, core.Plural(r.Warnings, "warning", "warnings")))
		}
		for _, s := range r.Severities {
			if s.Name != "error" && s.Name != "warning" {
				parts = append(parts, fmt.Sprintf("%d %s", s.Count, s.Name))
			}
		}
		line += fmt.Sprintf(" (%s) from %s", strings.Join(parts, ", "), core.TopCounts(r.Linters, maxLintersInNotice, "linter", "linters"))
	}
	if n := len(r.EnabledLinters); n > 0 {
		line += fmt.Sprintf("; %d %s enabled", n, core.Plural(n, "linter", "linters"))
	}
	if r.Fixable > 0 {
		line += fmt.Sprintf("; %d fixable", r.Fixable)
	}
	if r.RunErrors > 0 {
		line += fmt.Sprintf("; %d run %s", r.RunErrors, core.Plural(r.RunErrors, "error", "errors"))
	}
	if r.RunWarnings > 0 {
		line += fmt.Sprintf("; %d run %s", r.RunWarnings, core.Plural(r.RunWarnings, "warning", "warnings"))
	}
	return line
}

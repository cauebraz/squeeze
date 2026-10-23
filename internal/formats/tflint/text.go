package tflint

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

func text(r report, o options) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, d := range r.Problems {
		tfdiag.Write(&b, d, tfdiag.WriteOptions{NoDetail: o.NoDetail})
	}
	for _, is := range r.Issues {
		d := is.Diagnostic
		switch {
		case is.Fixed:
			d.Address += ", fixed"
		case is.Fixable:
			d.Address += ", fixable"
		}
		tfdiag.Write(&b, d, tfdiag.WriteOptions{NoDetail: o.NoDetail})
	}
	if r.IssuesOmitted > 0 {
		fmt.Fprintf(&b, "... %d more %s, %s (--max-issues 0)\n", r.IssuesOmitted, core.Plural(r.IssuesOmitted, "issue", "issues"), core.TopCounts(r.OmittedByRule, maxRulesInNotice, "rule", "rules"))
	}
	return b.String()
}

func header(r report) string {
	total := r.Errors + r.Warnings + r.Notices
	if total == 0 && len(r.Problems) == 0 {
		return "tflint: no issues"
	}
	parts := tfdiag.CountWords(r.Errors, r.Warnings)
	if r.Notices > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", r.Notices, core.Plural(r.Notices, "notice", "notices")))
	}
	line := fmt.Sprintf("tflint: %d %s", total, core.Plural(total, "issue", "issues"))
	if len(parts) > 0 {
		line += ", " + strings.Join(parts, ", ")
	}
	if r.Fixable > 0 {
		line += fmt.Sprintf("; %d fixable", r.Fixable)
	}
	if r.Fixed > 0 {
		line += fmt.Sprintf("; %d fixed", r.Fixed)
	}
	if r.ToolErrors > 0 {
		line += fmt.Sprintf("; %d tflint %s, not every check ran", r.ToolErrors, core.Plural(r.ToolErrors, "error", "errors"))
	}
	return line
}

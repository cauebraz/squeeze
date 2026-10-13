package tflint

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

func Text(r Report, o Options) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, d := range r.Diagnostics {
		tfdiag.Write(&b, d, tfdiag.WriteOptions{NoDetail: o.NoDetail})
	}
	if r.DiagnosticsOmitted > 0 {
		items := make([]string, 0, maxRulesInNotice+1)
		for i, rc := range r.OmittedRules {
			if i == maxRulesInNotice {
				rest := len(r.OmittedRules) - maxRulesInNotice
				items = append(items, fmt.Sprintf("%d other %s", rest, core.Plural(rest, "rule", "rules")))
				break
			}
			items = append(items, fmt.Sprintf("%s %d", rc.Rule, rc.Count))
		}
		fmt.Fprintf(&b, "... %d more %s, %s (--max-issues 0)\n", r.DiagnosticsOmitted, core.Plural(r.DiagnosticsOmitted, "issue", "issues"), strings.Join(items, ", "))
	}
	return b.String()
}

func header(r Report) string {
	total := r.Errors + r.Warnings + r.Notices
	if total == 0 && r.ToolErrors == 0 {
		return "tflint: no issues"
	}
	var parts []string
	for _, c := range []struct {
		n         int
		one, many string
	}{{r.Errors, "error", "errors"}, {r.Warnings, "warning", "warnings"}, {r.Notices, "notice", "notices"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, core.Plural(c.n, c.one, c.many)))
		}
	}
	line := fmt.Sprintf("tflint: %d %s", total, core.Plural(total, "issue", "issues"))
	if len(parts) > 0 {
		line += ": " + strings.Join(parts, ", ")
	}
	if r.Fixable > 0 {
		line += fmt.Sprintf("; %d fixable with tflint --fix", r.Fixable)
	}
	if r.Fixed > 0 {
		line += fmt.Sprintf("; %d fixed", r.Fixed)
	}
	if r.ToolErrors > 0 {
		line += fmt.Sprintf("; tflint reported %d %s and did not finish every check", r.ToolErrors, core.Plural(r.ToolErrors, "error", "errors"))
	}
	return line
}

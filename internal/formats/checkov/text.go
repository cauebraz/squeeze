package checkov

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

func Text(r Report, o Options) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, p := range r.ParseErrorPath {
		fmt.Fprintf(&b, "parse error: %s\n", p)
	}
	writeGroups(&b, r.FailedChecks, r.failedPerResource(), false)
	if r.ChecksOmitted > 0 {
		items := make([]string, 0, maxChecksInNotice+1)
		for i, cc := range r.OmittedChecks {
			if i == maxChecksInNotice {
				rest := len(r.OmittedChecks) - maxChecksInNotice
				items = append(items, fmt.Sprintf("%d other %s", rest, core.Plural(rest, "check", "checks")))
				break
			}
			items = append(items, fmt.Sprintf("%s %d", cc.CheckID, cc.Count))
		}
		fmt.Fprintf(&b, "... %d more failed %s, %s (--max-checks 0)\n", r.ChecksOmitted, core.Plural(r.ChecksOmitted, "check", "checks"), strings.Join(items, ", "))
	}
	switch {
	case o.All && len(r.SkippedChecks) > 0:
		b.WriteString("skipped:\n")
		writeGroups(&b, r.SkippedChecks, nil, true)
	case r.Skipped > 0:
		fmt.Fprintf(&b, "... %d skipped %s (--all)\n", r.Skipped, core.Plural(r.Skipped, "check", "checks"))
	}
	return b.String()
}

func header(r Report) string {
	tool := r.Tool
	if r.ToolVersion != "" {
		tool += " " + r.ToolVersion
	}
	if r.Passed+r.Failed+r.Skipped+r.ParsingErrors == 0 {
		return tool + ": nothing scanned"
	}
	line := fmt.Sprintf("%s: %d failed, %d passed, %d skipped", tool, r.Failed, r.Passed, r.Skipped)
	if r.ParsingErrors > 0 {
		line += fmt.Sprintf(", %d parsing %s", r.ParsingErrors, core.Plural(r.ParsingErrors, "error", "errors"))
	}
	line += fmt.Sprintf("; %d %s", r.Resources, core.Plural(r.Resources, "resource", "resources"))
	if len(r.Frameworks) > 1 {
		parts := make([]string, 0, len(r.Frameworks))
		for _, f := range r.Frameworks {
			parts = append(parts, fmt.Sprintf("%s %d failed", f.CheckType, f.Failed))
		}
		line += "; " + strings.Join(parts, ", ")
	}
	return line
}

func (r Report) failedPerResource() map[string]int {
	return r.PerResource
}

func writeGroups(b *strings.Builder, checks []Check, totals map[string]int, skipped bool) {
	last := ""
	for _, c := range checks {
		if k := c.key(); k != last {
			last = k
			fmt.Fprintf(b, "%s (%s", c.Resource, location(c.File, []int{c.Start, c.End}))
			if c.Caller != "" {
				b.WriteString(", via " + c.Caller)
			}
			b.WriteByte(')')
			if n := totals[k]; n > 0 {
				fmt.Fprintf(b, ": %d failed %s", n, core.Plural(n, "check", "checks"))
			}
			b.WriteByte('\n')
		}
		b.WriteString("  ")
		if c.Severity != "" {
			b.WriteString(c.Severity + " ")
		}
		b.WriteString(c.CheckID + " ")
		if skipped && c.Comment != "" {
			b.WriteString(c.Comment)
		} else {
			b.WriteString(c.Name)
		}
		b.WriteByte('\n')
	}
}

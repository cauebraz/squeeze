package ghactions

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

func Text(r Report) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, j := range r.Jobs {
		for _, s := range j.Steps {
			status := "step"
			if s.Failed {
				status = "fail"
			}
			fmt.Fprintf(&b, "%s %s: %s\n", status, label(j.Name), s.Name)
			for _, l := range s.Output {
				b.WriteString("  " + l + "\n")
			}
			for _, e := range s.Errors {
				b.WriteString("  error: " + e + "\n")
			}
			for _, w := range s.Warnings {
				b.WriteString("  warning: " + w + "\n")
			}
		}
	}
	if r.OtherSteps > 0 {
		fmt.Fprintf(&b, "... %d other %s (--all)\n", r.OtherSteps, core.Plural(r.OtherSteps, "step", "steps"))
	}
	return b.String()
}

func label(job string) string {
	if job == "" {
		return "job"
	}
	return job
}

func header(r Report) string {
	failed := []string{}
	for _, j := range r.Jobs {
		if j.Failed {
			failed = append(failed, label(j.Name))
		}
	}
	line := fmt.Sprintf("%s: %d %s", core.ToolLabel("github actions", r.ToolVersion), len(r.Jobs), core.Plural(len(r.Jobs), "job", "jobs"))
	if len(failed) > 0 {
		line += fmt.Sprintf(", %d failed", len(failed))
	}
	var parts []string
	if r.Errors > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", r.Errors, core.Plural(r.Errors, "error", "errors")))
	}
	if r.Warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", r.Warnings, core.Plural(r.Warnings, "warning", "warnings")))
	}
	if len(parts) > 0 {
		line += "; " + strings.Join(parts, ", ")
	}
	return line
}

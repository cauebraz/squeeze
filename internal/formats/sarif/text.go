package sarif

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
	for _, d := range r.Notifications {
		tfdiag.Write(&b, d, tfdiag.WriteOptions{NoDetail: o.NoDetail})
	}
	for _, res := range r.Results {
		tfdiag.Write(&b, res.Diagnostic, tfdiag.WriteOptions{NoDetail: o.NoDetail})
	}
	if r.ResultsOmitted > 0 {
		fmt.Fprintf(&b, "... %d more %s, %s (--max-results 0)\n", r.ResultsOmitted, core.Plural(r.ResultsOmitted, "result", "results"), core.TopCounts(r.OmittedByRule, maxRulesInNotice, "rule", "rules"))
	}
	return b.String()
}

func runWords(s RunSummary) string {
	line := core.ToolLabel(s.Tool, s.Version) + ", "
	if s.Results == 0 {
		return line + "no results"
	}
	var parts []string
	for _, c := range []struct {
		n         int
		one, many string
	}{{s.Errors, "error", "errors"}, {s.Warnings, "warning", "warnings"}, {s.Notes, "note", "notes"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, core.Plural(c.n, c.one, c.many)))
		}
	}
	return line + fmt.Sprintf("%d %s (%s)", s.Results, core.Plural(s.Results, "result", "results"), strings.Join(parts, ", "))
}

func header(r Report) string {
	runs := make([]string, 0, len(r.Runs))
	for _, s := range r.Runs {
		runs = append(runs, runWords(s))
	}
	line := "sarif: no runs"
	if len(runs) > 0 {
		line = "sarif: " + strings.Join(runs, "; ")
	}
	if r.Suppressed > 0 {
		line += fmt.Sprintf("; %d suppressed", r.Suppressed)
	}
	if len(r.OtherKinds) > 0 {
		total := 0
		for _, k := range r.OtherKinds {
			total += k.Count
		}
		line += fmt.Sprintf("; %d not failures: %s", total, core.TopCounts(r.OtherKinds, len(r.OtherKinds), "kind", "kinds"))
	}
	if r.ToolErrors > 0 {
		line += fmt.Sprintf("; %d tool %s, not every check ran", r.ToolErrors, core.Plural(r.ToolErrors, "error", "errors"))
	}
	return line
}

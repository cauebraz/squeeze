package eslint

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
	for _, d := range r.ParseErrors {
		tfdiag.Write(&b, d, tfdiag.WriteOptions{})
	}
	for _, p := range r.Problems {
		d := p.Diagnostic
		if p.Fixable {
			if d.Address != "" {
				d.Address += ", "
			}
			d.Address += "fixable"
		}
		tfdiag.Write(&b, d, tfdiag.WriteOptions{})
	}
	if r.ProblemsOmitted > 0 {
		fmt.Fprintf(&b, "... %d more %s, %s (--max-problems 0)\n", r.ProblemsOmitted, core.Plural(r.ProblemsOmitted, "problem", "problems"), core.TopCounts(r.OmittedByRule, maxRulesInNotice, "rule", "rules"))
	}
	return b.String()
}

func header(r Report) string {
	total := r.Errors + r.Warnings
	line := fmt.Sprintf("eslint: %d %s", total, core.Plural(total, "problem", "problems"))
	if total > 0 {
		line += fmt.Sprintf(" (%d %s, %d %s) in %d of %d %s", r.Errors, core.Plural(r.Errors, "error", "errors"), r.Warnings, core.Plural(r.Warnings, "warning", "warnings"), r.FilesWithProblem, r.Files, core.Plural(r.Files, "file", "files"))
	} else {
		line += fmt.Sprintf(" in %d %s", r.Files, core.Plural(r.Files, "file", "files"))
	}
	if r.Fatal > 0 {
		line += fmt.Sprintf("; %d %s not parsed", r.Fatal, core.Plural(r.Fatal, "file", "files"))
	}
	if r.Fixable > 0 {
		line += fmt.Sprintf("; %d fixable with --fix", r.Fixable)
	}
	if r.Suppressed > 0 {
		line += fmt.Sprintf("; %d suppressed", r.Suppressed)
	}
	return line
}

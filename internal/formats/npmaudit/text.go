package npmaudit

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

const maxSeveritiesInNotice = 5

func Text(r Report) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, e := range r.Vulnerabilities {
		b.WriteString(line(e))
		b.WriteByte('\n')
	}
	if r.VulnerabilitiesOmitted > 0 {
		fmt.Fprintf(&b, "... %d more %s, %s (--max-vulnerabilities 0)\n", r.VulnerabilitiesOmitted,
			core.Plural(r.VulnerabilitiesOmitted, "vulnerability", "vulnerabilities"),
			core.TopCounts(r.OmittedBySeverity, maxSeveritiesInNotice, "severity", "severities"))
	}
	return b.String()
}

func header(r Report) string {
	s := fmt.Sprintf("npm audit: %d %s", r.Total, core.Plural(r.Total, "vulnerability", "vulnerabilities"))
	if r.Total > 0 {
		parts := make([]string, 0, len(r.BySeverity))
		for _, c := range r.BySeverity {
			parts = append(parts, fmt.Sprintf("%d %s", c.Count, c.Name))
		}
		s += " (" + strings.Join(parts, ", ") + ")"
	}
	s += fmt.Sprintf(" in %d %s", r.Dependencies.Total, core.Plural(r.Dependencies.Total, "dependency", "dependencies"))
	if r.Total == 0 {
		return s
	}
	s += fmt.Sprintf("; %d direct", r.Direct)
	var fixes []string
	if r.Fixes.Auto > 0 {
		fixes = append(fixes, fmt.Sprintf("%d with npm audit fix", r.Fixes.Auto))
	}
	if force := r.Fixes.Force + r.Fixes.Breaking; force > 0 {
		f := fmt.Sprintf("%d with npm audit fix --force", force)
		if r.Fixes.Breaking > 0 {
			f += fmt.Sprintf(" (%d breaking)", r.Fixes.Breaking)
		}
		fixes = append(fixes, f)
	}
	if r.Fixes.None > 0 {
		fixes = append(fixes, fmt.Sprintf("%d without one", r.Fixes.None))
	}
	return s + "; fix: " + strings.Join(fixes, ", ")
}

func fix(f Fix) string {
	switch {
	case !f.Available:
		return "no fix"
	case !f.Force:
		return "fix with npm audit fix"
	}
	s := "fix " + f.Name + "@" + f.Version + " (--force"
	if f.SemVerMajor {
		s += ", breaking"
	}
	return s + ")"
}

func line(e Entry) string {
	s := e.Severity + " " + e.Name
	if e.Range != "" {
		s += " " + e.Range
	}
	if e.Direct {
		s += " direct"
	}
	s += ": " + fix(e.Fix)
	var ids []string
	for _, a := range e.Advisories {
		if !slices.Contains(ids, a.ID) {
			ids = append(ids, a.ID)
		}
	}
	switch {
	case len(ids) > 0:
		s += "; " + strings.Join(ids, ", ")
	case len(e.Via) > 0:
		s += "; via " + strings.Join(e.Via, ", ")
	}
	if !e.Direct && !e.Fix.Force && len(e.Effects) > 0 {
		s += "; required by " + strings.Join(e.Effects, ", ")
	}
	return s
}

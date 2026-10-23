package trivy

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const maxTitleBytes = 100

func Text(r Report, o Options) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, res := range r.Resources {
		if res.Error != "" {
			b.WriteString("error: " + res.Name + ": " + res.Error + "\n")
		}
	}
	for _, d := range r.Diagnostics {
		tfdiag.Write(&b, d, tfdiag.WriteOptions{})
	}
	writeTargets(&b, r, r.Findings)
	if r.FindingsOmitted > 0 {
		var parts []string
		for _, s := range severities {
			if n := r.OmittedBySeverity[s]; n > 0 {
				parts = append(parts, fmt.Sprintf("%d %s", n, s))
			}
		}
		fmt.Fprintf(&b, "... %d more %s, %s (--max-findings 0)\n", r.FindingsOmitted, core.Plural(r.FindingsOmitted, "finding", "findings"), strings.Join(parts, ", "))
	}
	listed := r.Findings
	if len(r.Hidden) > 0 {
		if o.All {
			b.WriteString("also reported:\n")
			writeTargets(&b, r, r.Hidden)
			listed = append(append([]Finding(nil), r.Findings...), r.Hidden...)
		} else {
			var parts []string
			if r.Exceptions > 0 {
				parts = append(parts, fmt.Sprintf("%d misconfiguration %s", r.Exceptions, core.Plural(r.Exceptions, "exception", "exceptions")))
			}
			if n := r.Licenses - r.SevereLicenses; n > 0 {
				parts = append(parts, fmt.Sprintf("%d %s below HIGH", n, core.Plural(n, "license", "licenses")))
			}
			fmt.Fprintf(&b, "... %s (--all)\n", strings.Join(parts, ", "))
		}
	}
	writeFixes(&b, listed)
	return b.String()
}

func writeFixes(b *strings.Builder, findings []Finding) {
	seen := map[string]bool{}
	for _, f := range findings {
		if f.Kind != "misconfiguration" || f.Fix == "" || seen[f.ID] {
			continue
		}
		if len(seen) == 0 {
			b.WriteString("fixes:\n")
		}
		seen[f.ID] = true
		b.WriteString("  " + f.ID + ": " + title(f.Fix) + "\n")
	}
}

func header(r Report) string {
	line := core.ToolLabel(r.Tool, r.ToolVersion)
	if r.Cluster != nil {
		line += ": cluster"
		if *r.Cluster != "" {
			line += " " + *r.Cluster
		}
		line += fmt.Sprintf(", %d %s", len(r.Resources), core.Plural(len(r.Resources), "resource", "resources"))
	} else if r.Artifact != "" {
		line += ": " + r.Artifact
		if r.ArtifactType != "" {
			line += " (" + r.ArtifactType
			if r.OS != "" {
				line += ", " + r.OS
			}
			line += ")"
		}
	}
	var parts []string
	if r.Vulnerabilities > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", r.Vulnerabilities, core.Plural(r.Vulnerabilities, "vulnerability", "vulnerabilities")))
	}
	if r.Misconfigurations > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", r.Misconfigurations, core.Plural(r.Misconfigurations, "misconfiguration", "misconfigurations")))
	}
	if r.Secrets > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", r.Secrets, core.Plural(r.Secrets, "secret", "secrets")))
	}
	if r.Licenses > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", r.Licenses, core.Plural(r.Licenses, "license", "licenses")))
	}
	if len(parts) == 0 {
		line += ": no findings"
	} else {
		line += ": " + strings.Join(parts, ", ")
		var sev []string
		for _, s := range severities {
			if n := r.BySeverity[s]; n > 0 {
				sev = append(sev, fmt.Sprintf("%d %s", n, s))
			}
		}
		if len(sev) > 0 {
			line += "; " + strings.Join(sev, ", ")
		}
	}
	if r.Fixable > 0 {
		line += fmt.Sprintf("; %d fixable", r.Fixable)
	}
	if r.ChecksPassed > 0 {
		line += fmt.Sprintf("; %d misconfiguration %s passed", r.ChecksPassed, core.Plural(r.ChecksPassed, "check", "checks"))
	}
	return line
}

func writeTargets(b *strings.Builder, r Report, findings []Finding) {
	last, resource, indent := -1, "", ""
	if r.Cluster != nil {
		indent = "  "
	}
	for _, f := range findings {
		if f.TargetIdx != last {
			t := r.Targets[f.TargetIdx]
			if r.Cluster != nil && (last == -1 || t.Resource != resource) {
				resource = t.Resource
				b.WriteString(resource + ":\n")
			}
			last = f.TargetIdx
			b.WriteString(indent + f.Target)
			if t.Type != "" {
				b.WriteString(" (" + t.Type + ")")
			} else if t.Class != "" {
				b.WriteString(" (" + t.Class + ")")
			}
			b.WriteString(":\n")
		}
		writeFinding(b, indent, f)
	}
}

func writeFinding(b *strings.Builder, indent string, f Finding) {
	b.WriteString(indent + "  " + f.Severity + " " + f.ID)
	switch f.Kind {
	case "vulnerability":
		b.WriteString(" " + f.Package + " " + f.Installed)
		if f.Fixed != "" {
			b.WriteString(" -> " + f.Fixed)
		} else {
			status := f.Status
			if status == "" {
				status = "no fix"
			}
			b.WriteString(" (" + strings.ReplaceAll(status, "_", " ") + ")")
		}
	case "misconfiguration":
		if f.Resource != "" {
			b.WriteString(" " + f.Resource)
		}
		b.WriteString(lines(f.StartLine, f.EndLine))
		if strings.EqualFold(f.Status, "EXCEPTION") {
			b.WriteString(" (exception)")
		}
	case "secret":
		b.WriteString(lines(f.StartLine, f.EndLine))
	case "license":
		b.WriteString(" " + f.Package)
	}
	if f.Title != "" {
		b.WriteString(": " + title(f.Title))
	}
	b.WriteByte('\n')
}

func lines(start, end int) string {
	switch {
	case start <= 0:
		return ""
	case end <= start:
		return fmt.Sprintf(" line %d", start)
	default:
		return fmt.Sprintf(" lines %d-%d", start, end)
	}
}

func title(s string) string {
	return core.Ellipsis(s, maxTitleBytes)
}

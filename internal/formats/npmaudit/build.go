package npmaudit

import (
	"cmp"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/sanitize"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

var severities = core.SeverityOrder{"critical", "high", "moderate", "low", "info"}

type advisory struct {
	ID       string   `json:"id"`
	Source   int      `json:"source"`
	Title    string   `json:"title"`
	URL      string   `json:"url"`
	Severity string   `json:"severity"`
	Range    string   `json:"range"`
	CWE      []string `json:"cwe"`
	CVSS     float64  `json:"cvss_score"`
}

type entry struct {
	Name       string     `json:"name"`
	Severity   string     `json:"severity"`
	Range      string     `json:"range"`
	Direct     bool       `json:"direct"`
	Fix        fix        `json:"fix"`
	Advisories []advisory `json:"advisories"`
	Via        []string   `json:"via"`
	Effects    []string   `json:"effects"`
	Nodes      []string   `json:"nodes"`
	key        string
}

type fix struct {
	Available   bool   `json:"available"`
	Force       bool   `json:"force,omitempty"`
	Name        string `json:"name,omitempty"`
	Version     string `json:"version,omitempty"`
	SemVerMajor bool   `json:"semver_major,omitempty"`
}

type fixCounts struct {
	Auto     int `json:"auto"`
	Force    int `json:"force"`
	Breaking int `json:"breaking"`
	None     int `json:"none"`
}

type report struct {
	Tool                   string              `json:"tool"`
	ToolVersion            string              `json:"tool_version"`
	AuditReportVersion     int                 `json:"audit_report_version"`
	Dependencies           dependencies        `json:"dependencies"`
	Total                  int                 `json:"total"`
	BySeverity             []core.Count        `json:"by_severity"`
	Direct                 int                 `json:"direct"`
	Fixes                  fixCounts           `json:"fixes"`
	Vulnerabilities        []entry             `json:"vulnerabilities"`
	VulnerabilitiesOmitted int                 `json:"vulnerabilities_omitted"`
	OmittedBySeverity      []core.Count        `json:"omitted_by_severity"`
	Diagnostics            []tfdiag.Diagnostic `json:"diagnostics,omitempty"`
}

func cleanAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, sanitize.Clean(s))
	}
	return out
}

func advisoryID(a *rawAdvisory) string {
	if id := sanitize.Clean(path.Base(a.URL)); a.URL != "" && id != "" && id != "." && id != "/" {
		return id
	}
	if a.Source != 0 {
		return strconv.Itoa(a.Source)
	}
	return "advisory"
}

func convertFix(f rawFix) fix {
	return fix{Available: f.Available, Force: f.Force, Name: sanitize.Clean(f.Name), Version: sanitize.Clean(f.Version), SemVerMajor: f.IsSemVerMajor}
}

func toEntry(key string, v rawVulnerability) entry {
	e := entry{Name: sanitize.Clean(v.Name), Severity: sanitize.Clean(v.Severity), Range: sanitize.Clean(v.Range), Direct: v.IsDirect, Fix: convertFix(v.FixAvailable),
		Advisories: []advisory{}, Via: []string{}, Effects: cleanAll(v.Effects), Nodes: cleanAll(v.Nodes), key: key}
	if e.Name == "" {
		e.Name = sanitize.Clean(key)
	}
	if !severities.Known(e.Severity) {
		e.Severity = "unknown"
	}
	for _, via := range v.Via {
		switch {
		case via.Advisory != nil:
			a := via.Advisory
			e.Advisories = append(e.Advisories, advisory{ID: advisoryID(a), Source: a.Source, Title: sanitize.Clean(a.Title), URL: sanitize.Clean(a.URL),
				Severity: sanitize.Clean(a.Severity), Range: sanitize.Clean(a.Range), CWE: cleanAll(a.CWE), CVSS: a.CVSS.Score})
		case via.Package != "":
			e.Via = append(e.Via, sanitize.Clean(via.Package))
		}
	}
	slices.SortStableFunc(e.Advisories, func(a, b advisory) int {
		return cmp.Compare(severities.Rank(a.Severity), severities.Rank(b.Severity))
	})
	return e
}

func bySeverity(entries []entry) []core.Count {
	counts := map[string]int{}
	for _, e := range entries {
		counts[e.Severity]++
	}
	out := core.SortedCounts(counts)
	slices.SortStableFunc(out, func(a, b core.Count) int { return cmp.Compare(severities.Rank(a.Name), severities.Rank(b.Name)) })
	return out
}

func build(a *audit, o options) report {
	r := report{Tool: "npm", AuditReportVersion: a.Version, Dependencies: a.Dependencies, Vulnerabilities: []entry{}}
	r.Diagnostics = tfdiag.Normalize(tfdiag.FromLines(a.Trailing, ""))
	entries := make([]entry, 0, len(a.Vulnerabilities))
	for key, v := range a.Vulnerabilities {
		e := toEntry(key, v)
		entries = append(entries, e)
		if e.Direct {
			r.Direct++
		}
		switch {
		case !e.Fix.Available:
			r.Fixes.None++
		case e.Fix.SemVerMajor:
			r.Fixes.Breaking++
		case e.Fix.Force:
			r.Fixes.Force++
		default:
			r.Fixes.Auto++
		}
	}
	slices.SortFunc(entries, func(a, b entry) int {
		return cmp.Or(cmp.Compare(severities.Rank(a.Severity), severities.Rank(b.Severity)), strings.Compare(a.Name, b.Name), strings.Compare(a.key, b.key))
	})
	r.Total = len(entries)
	r.BySeverity = bySeverity(entries)
	kept, omitted := core.Limit(entries, o.maxVulnerabilities)
	r.Vulnerabilities = append(r.Vulnerabilities, kept...)
	r.VulnerabilitiesOmitted = omitted
	r.OmittedBySeverity = bySeverity(entries[len(kept):])
	return r
}

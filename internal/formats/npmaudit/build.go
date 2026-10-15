package npmaudit

import (
	"path"
	"slices"
	"sort"
	"strconv"

	"github.com/cauebraz/squeeze/internal/core"
)

var severities = []string{"critical", "high", "moderate", "low", "info"}

func rank(s string) int {
	if i := slices.Index(severities, s); i >= 0 {
		return len(severities) - 1 - i
	}
	return len(severities)
}

type AdvisoryData struct {
	ID       string   `json:"id"`
	Source   int      `json:"source"`
	Title    string   `json:"title"`
	URL      string   `json:"url"`
	Severity string   `json:"severity"`
	Range    string   `json:"range"`
	CWE      []string `json:"cwe"`
	CVSS     float64  `json:"cvss_score"`
}

type Entry struct {
	Name       string         `json:"name"`
	Severity   string         `json:"severity"`
	Range      string         `json:"range"`
	Direct     bool           `json:"direct"`
	Fix        Fix            `json:"fix"`
	Advisories []AdvisoryData `json:"advisories"`
	Via        []string       `json:"via"`
	Effects    []string       `json:"effects"`
	Nodes      []string       `json:"nodes"`
	key        string
}

type Fixes struct {
	Auto     int `json:"auto"`
	Force    int `json:"force"`
	Breaking int `json:"breaking"`
	None     int `json:"none"`
}

type Report struct {
	Tool                   string       `json:"tool"`
	ToolVersion            string       `json:"tool_version"`
	AuditReportVersion     int          `json:"audit_report_version"`
	Dependencies           Dependencies `json:"dependencies"`
	Total                  int          `json:"total"`
	BySeverity             []core.Count `json:"by_severity"`
	Direct                 int          `json:"direct"`
	Fixes                  Fixes        `json:"fixes"`
	Vulnerabilities        []Entry      `json:"vulnerabilities"`
	VulnerabilitiesOmitted int          `json:"vulnerabilities_omitted"`
	OmittedBySeverity      []core.Count `json:"omitted_by_severity"`
}

func cleanAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, core.Clean(s))
	}
	return out
}

func advisoryID(a *Advisory) string {
	if id := core.Clean(path.Base(a.URL)); a.URL != "" && id != "" && id != "." && id != "/" {
		return id
	}
	if a.Source != 0 {
		return strconv.Itoa(a.Source)
	}
	return "advisory"
}

func entry(key string, v Vulnerability) Entry {
	e := Entry{Name: core.Clean(v.Name), Severity: core.Clean(v.Severity), Range: core.Clean(v.Range), Direct: v.IsDirect, Fix: v.FixAvailable,
		Advisories: []AdvisoryData{}, Via: []string{}, Effects: cleanAll(v.Effects), Nodes: cleanAll(v.Nodes), key: key}
	if e.Name == "" {
		e.Name = core.Clean(key)
	}
	if !slices.Contains(severities, e.Severity) {
		e.Severity = "unknown"
	}
	e.Fix.Name, e.Fix.Version = core.Clean(e.Fix.Name), core.Clean(e.Fix.Version)
	for _, via := range v.Via {
		switch {
		case via.Advisory != nil:
			a := via.Advisory
			e.Advisories = append(e.Advisories, AdvisoryData{ID: advisoryID(a), Source: a.Source, Title: core.Clean(a.Title), URL: core.Clean(a.URL),
				Severity: core.Clean(a.Severity), Range: core.Clean(a.Range), CWE: cleanAll(a.CWE), CVSS: a.CVSS.Score})
		case via.Package != "":
			e.Via = append(e.Via, core.Clean(via.Package))
		}
	}
	sort.SliceStable(e.Advisories, func(i, j int) bool { return rank(e.Advisories[i].Severity) > rank(e.Advisories[j].Severity) })
	return e
}

func bySeverity(entries []Entry) []core.Count {
	counts := map[string]int{}
	for _, e := range entries {
		counts[e.Severity]++
	}
	out := core.SortedCounts(counts)
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i].Name) > rank(out[j].Name) })
	return out
}

func Build(a *Audit, o Options) Report {
	r := Report{Tool: "npm", AuditReportVersion: a.Version, Dependencies: a.Dependencies, Vulnerabilities: []Entry{}}
	entries := make([]Entry, 0, len(a.Vulnerabilities))
	for key, v := range a.Vulnerabilities {
		e := entry(key, v)
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
	sort.Slice(entries, func(i, j int) bool {
		if ri, rj := rank(entries[i].Severity), rank(entries[j].Severity); ri != rj {
			return ri > rj
		}
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].key < entries[j].key
	})
	r.Total = len(entries)
	r.BySeverity = bySeverity(entries)
	kept, omitted := core.Limit(entries, o.MaxVulnerabilities)
	r.Vulnerabilities = append(r.Vulnerabilities, kept...)
	r.VulnerabilitiesOmitted = omitted
	r.OmittedBySeverity = bySeverity(entries[len(kept):])
	return r
}

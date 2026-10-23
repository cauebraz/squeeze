package trivy

import (
	"cmp"
	"slices"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/sanitize"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

var severities = core.SeverityOrder{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN"}

func severity(s string) string {
	s = strings.ToUpper(sanitize.Clean(s))
	if !severities.Known(s) {
		return "UNKNOWN"
	}
	return s
}

type findingKind string

const (
	kindVulnerability    findingKind = "vulnerability"
	kindMisconfiguration findingKind = "misconfiguration"
	kindSecret           findingKind = "secret"
	kindLicense          findingKind = "license"
)

type misconfStatus string

const (
	misconfPass      misconfStatus = "PASS"
	misconfFail      misconfStatus = "FAIL"
	misconfException misconfStatus = "EXCEPTION"
)

func statusOf(s string) misconfStatus {
	return misconfStatus(strings.ToUpper(s))
}

type finding struct {
	Kind      findingKind `json:"kind"`
	Target    string      `json:"target"`
	TargetIdx int         `json:"target_index"`
	Severity  string      `json:"severity"`
	ID        string      `json:"id"`
	Title     string      `json:"title,omitempty"`
	Package   string      `json:"package,omitempty"`
	Installed string      `json:"installed_version,omitempty"`
	Fixed     string      `json:"fixed_version,omitempty"`
	Status    string      `json:"status,omitempty"`
	Resource  string      `json:"resource,omitempty"`
	Message   string      `json:"message,omitempty"`
	Fix       string      `json:"resolution,omitempty"`
	StartLine int         `json:"start_line,omitempty"`
	EndLine   int         `json:"end_line,omitempty"`
	URL       string      `json:"url,omitempty"`
	order     int
	rank      int
}

type target struct {
	Name     string `json:"name"`
	Resource string `json:"resource,omitempty"`
	Class    string `json:"class,omitempty"`
	Type     string `json:"type,omitempty"`
	Findings int    `json:"findings"`
	Passed   int    `json:"misconfig_checks_passed,omitempty"`
}

type resourceSummary struct {
	Name     string `json:"name"`
	Findings int    `json:"findings"`
	Error    string `json:"error,omitempty"`
}

type report struct {
	Tool              string              `json:"tool"`
	ToolVersion       string              `json:"tool_version"`
	Cluster           *string             `json:"cluster,omitempty"`
	Resources         []resourceSummary   `json:"resources,omitempty"`
	Artifact          string              `json:"artifact"`
	ArtifactType      string              `json:"artifact_type"`
	OS                string              `json:"os,omitempty"`
	Vulnerabilities   int                 `json:"vulnerabilities"`
	Misconfigurations int                 `json:"misconfigurations"`
	Secrets           int                 `json:"secrets"`
	Licenses          int                 `json:"licenses"`
	SevereLicenses    int                 `json:"severe_licenses"`
	Exceptions        int                 `json:"misconfig_exceptions"`
	ChecksPassed      int                 `json:"misconfig_checks_passed"`
	Fixable           int                 `json:"fixable"`
	BySeverity        map[string]int      `json:"by_severity"`
	Targets           []target            `json:"targets"`
	Findings          []finding           `json:"findings"`
	FindingsOmitted   int                 `json:"findings_omitted"`
	OmittedBySeverity map[string]int      `json:"omitted_by_severity"`
	Hidden            []finding           `json:"hidden"`
	HiddenBySeverity  map[string]int      `json:"hidden_by_severity"`
	Diagnostics       []tfdiag.Diagnostic `json:"diagnostics,omitempty"`
	all               bool
}

func build(raw *rawReport, o options) report {
	r := report{
		Tool:              "trivy",
		ToolVersion:       sanitize.Clean(raw.Trivy.Version),
		Artifact:          sanitize.Clean(raw.ArtifactName),
		ArtifactType:      sanitize.Clean(raw.ArtifactType),
		BySeverity:        map[string]int{},
		Targets:           []target{},
		Findings:          []finding{},
		OmittedBySeverity: map[string]int{},
		Hidden:            []finding{},
		HiddenBySeverity:  map[string]int{},
		all:               o.all,
	}
	if system := raw.Metadata.OS; system != nil {
		r.OS = strings.TrimSpace(sanitize.Clean(system.Family) + " " + sanitize.Clean(system.Name))
	}
	r.Diagnostics = tfdiag.Normalize(tfdiag.FromLines(raw.Trailing, ""))
	results, owners := raw.Results, make([]int, len(raw.Results))
	if raw.ClusterName != nil {
		cluster := sanitize.Clean(*raw.ClusterName)
		r.Cluster = &cluster
		r.Resources, results, owners = resources(raw)
	}
	var findings []finding
	for ti, res := range results {
		t := target{Name: sanitize.Clean(res.Target), Class: sanitize.Clean(res.Class), Type: sanitize.Clean(res.Type)}
		if r.Cluster != nil {
			t.Resource = r.Resources[owners[ti]].Name
		}
		add := func(f finding) {
			f.Target, f.TargetIdx, f.order, f.rank = t.Name, ti, ti, severities.Rank(f.Severity)
			findings = append(findings, f)
			r.BySeverity[f.Severity]++
			t.Findings++
		}
		hide := func(f finding) {
			f.Target, f.TargetIdx, f.order, f.rank = t.Name, ti, ti, severities.Rank(f.Severity)
			r.HiddenBySeverity[f.Severity]++
			r.Hidden = append(r.Hidden, f)
		}
		for _, v := range res.Vulnerabilities {
			f := finding{Kind: kindVulnerability, Severity: severity(v.Severity), ID: sanitize.Clean(v.VulnerabilityID), Title: sanitize.Clean(v.Title),
				Package: sanitize.Clean(v.PkgName), Installed: sanitize.Clean(v.InstalledVersion), Fixed: sanitize.Clean(v.FixedVersion), Status: sanitize.Clean(v.Status), URL: sanitize.Clean(v.PrimaryURL)}
			r.Vulnerabilities++
			if f.Fixed != "" {
				r.Fixable++
			}
			add(f)
		}
		if res.MisconfSummary != nil {
			t.Passed = res.MisconfSummary.Successes
			r.ChecksPassed += t.Passed
		}
		for _, m := range res.Misconfigurations {
			f := finding{Kind: kindMisconfiguration, Severity: severity(m.Severity), ID: sanitize.Clean(m.ID), Title: sanitize.Clean(m.Title), Message: sanitize.Clean(m.Message), Fix: sanitize.Clean(m.Resolution),
				Resource: sanitize.Clean(m.CauseMetadata.Resource), StartLine: m.CauseMetadata.StartLine, EndLine: m.CauseMetadata.EndLine, URL: sanitize.Clean(m.PrimaryURL), Status: sanitize.Clean(m.Status)}
			if f.ID == "" {
				f.ID = sanitize.Clean(m.AVDID)
			}
			switch statusOf(f.Status) {
			case misconfPass:
				continue
			case misconfException:
				r.Exceptions++
				hide(f)
				continue
			case misconfFail:
			default:
			}
			r.Misconfigurations++
			add(f)
		}
		for _, s := range res.Secrets {
			r.Secrets++
			add(finding{Kind: kindSecret, Severity: severity(s.Severity), ID: sanitize.Clean(s.RuleID), Title: sanitize.Clean(s.Title), StartLine: s.StartLine, EndLine: s.EndLine})
		}
		for _, l := range res.Licenses {
			f := finding{Kind: kindLicense, Severity: severity(l.Severity), ID: sanitize.Clean(l.Name), Title: sanitize.Clean(l.Category), Package: sanitize.Clean(l.PkgName), Resource: sanitize.Clean(l.FilePath)}
			r.Licenses++
			if severities.AtLeast(f.Severity, "HIGH") {
				r.SevereLicenses++
				add(f)
				continue
			}
			hide(f)
		}
		if r.Cluster != nil {
			r.Resources[owners[ti]].Findings += t.Findings
		}
		r.Targets = append(r.Targets, t)
	}
	slices.SortStableFunc(findings, func(a, b finding) int { return compareFindings(a, b, true) })
	kept, omitted := core.Limit(findings, o.maxFindings)
	r.FindingsOmitted = omitted
	for _, f := range findings[len(kept):] {
		r.OmittedBySeverity[f.Severity]++
	}
	slices.SortStableFunc(kept, func(a, b finding) int { return compareFindings(a, b, false) })
	r.Findings = append(r.Findings, kept...)
	slices.SortStableFunc(r.Hidden, func(a, b finding) int { return compareFindings(a, b, false) })
	return r
}

func resources(raw *rawReport) ([]resourceSummary, []rawResult, []int) {
	var groups []resourceSummary
	var members [][]rawResult
	index := map[string]int{}
	for _, k := range slices.Concat(raw.Resources, raw.Findings) {
		var parts []string
		for _, p := range []string{k.Namespace, k.Kind, k.Name} {
			if p = sanitize.Clean(p); p != "" {
				parts = append(parts, p)
			}
		}
		name := strings.Join(parts, "/")
		i, ok := index[name]
		if !ok {
			i = len(groups)
			index[name] = i
			groups = append(groups, resourceSummary{Name: name})
			members = append(members, nil)
		}
		if e := strings.Join(strings.Fields(sanitize.Clean(k.Error)), " "); e != "" {
			if groups[i].Error != "" {
				e = groups[i].Error + "; " + e
			}
			groups[i].Error = e
		}
		members[i] = append(members[i], k.Results...)
	}
	var results []rawResult
	var owners []int
	for i, m := range members {
		results = append(results, m...)
		for range m {
			owners = append(owners, i)
		}
	}
	return groups, results, owners
}

func compareFindings(a, b finding, severityFirst bool) int {
	bySeverity := cmp.Compare(a.rank, b.rank)
	first := 0
	if severityFirst {
		first = bySeverity
	}
	return cmp.Or(
		first,
		cmp.Compare(a.order, b.order),
		cmp.Compare(a.Kind, b.Kind),
		bySeverity,
		strings.Compare(a.Package, b.Package),
		strings.Compare(a.Resource, b.Resource),
		cmp.Compare(a.StartLine, b.StartLine),
		strings.Compare(a.ID, b.ID),
	)
}

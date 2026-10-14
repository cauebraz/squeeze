package trivy

import (
	"sort"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

var severities = []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "UNKNOWN"}

func rank(s string) int {
	for i, v := range severities {
		if s == v {
			return i
		}
	}
	return len(severities)
}

func severity(s string) string {
	s = strings.ToUpper(core.Clean(s))
	if rank(s) == len(severities) {
		return "UNKNOWN"
	}
	return s
}

type Finding struct {
	Kind      string `json:"kind"`
	Target    string `json:"target"`
	TargetIdx int    `json:"target_index"`
	Severity  string `json:"severity"`
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
	Package   string `json:"package,omitempty"`
	Installed string `json:"installed_version,omitempty"`
	Fixed     string `json:"fixed_version,omitempty"`
	Status    string `json:"status,omitempty"`
	Resource  string `json:"resource,omitempty"`
	Message   string `json:"message,omitempty"`
	Fix       string `json:"resolution,omitempty"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	URL       string `json:"url,omitempty"`
	order     int
}

type Target struct {
	Name     string `json:"name"`
	Class    string `json:"class,omitempty"`
	Type     string `json:"type,omitempty"`
	Findings int    `json:"findings"`
	Passed   int    `json:"misconfig_checks_passed,omitempty"`
}

type Report struct {
	Tool              string         `json:"tool"`
	ToolVersion       string         `json:"tool_version"`
	Artifact          string         `json:"artifact"`
	ArtifactType      string         `json:"artifact_type"`
	OS                string         `json:"os,omitempty"`
	Vulnerabilities   int            `json:"vulnerabilities"`
	Misconfigurations int            `json:"misconfigurations"`
	Secrets           int            `json:"secrets"`
	Licenses          int            `json:"licenses"`
	SevereLicenses    int            `json:"severe_licenses"`
	Exceptions        int            `json:"misconfig_exceptions"`
	ChecksPassed      int            `json:"misconfig_checks_passed"`
	Fixable           int            `json:"fixable"`
	BySeverity        map[string]int `json:"by_severity"`
	Targets           []Target       `json:"targets"`
	Findings          []Finding      `json:"findings"`
	FindingsOmitted   int            `json:"findings_omitted"`
	OmittedBySeverity map[string]int `json:"omitted_by_severity"`
	Hidden            []Finding      `json:"hidden"`
}

func Build(raw *RawReport, o Options) Report {
	r := Report{
		Tool:              "trivy",
		ToolVersion:       core.Clean(raw.Trivy.Version),
		Artifact:          core.Clean(raw.ArtifactName),
		ArtifactType:      core.Clean(raw.ArtifactType),
		BySeverity:        map[string]int{},
		Targets:           []Target{},
		Findings:          []Finding{},
		OmittedBySeverity: map[string]int{},
		Hidden:            []Finding{},
	}
	if os := raw.Metadata.OS; os != nil {
		r.OS = strings.TrimSpace(core.Clean(os.Family) + " " + core.Clean(os.Name))
	}
	var findings []Finding
	for ti, res := range raw.Results {
		t := Target{Name: core.Clean(res.Target), Class: core.Clean(res.Class), Type: core.Clean(res.Type)}
		add := func(f Finding) {
			f.Target, f.TargetIdx, f.order = t.Name, ti, ti
			findings = append(findings, f)
			r.BySeverity[f.Severity]++
			t.Findings++
		}
		hide := func(f Finding) {
			f.Target, f.TargetIdx, f.order = t.Name, ti, ti
			r.BySeverity[f.Severity]++
			r.Hidden = append(r.Hidden, f)
		}
		for _, v := range res.Vulnerabilities {
			f := Finding{Kind: "vulnerability", Severity: severity(v.Severity), ID: core.Clean(v.VulnerabilityID), Title: core.Clean(v.Title),
				Package: core.Clean(v.PkgName), Installed: core.Clean(v.InstalledVersion), Fixed: core.Clean(v.FixedVersion), Status: core.Clean(v.Status), URL: core.Clean(v.PrimaryURL)}
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
			f := Finding{Kind: "misconfiguration", Severity: severity(m.Severity), ID: core.Clean(m.ID), Title: core.Clean(m.Title), Message: core.Clean(m.Message), Fix: core.Clean(m.Resolution),
				Resource: core.Clean(m.CauseMetadata.Resource), StartLine: m.CauseMetadata.StartLine, EndLine: m.CauseMetadata.EndLine, URL: core.Clean(m.PrimaryURL), Status: core.Clean(m.Status)}
			if f.ID == "" {
				f.ID = core.Clean(m.AVDID)
			}
			switch strings.ToUpper(f.Status) {
			case "PASS":
				continue
			case "EXCEPTION":
				r.Exceptions++
				hide(f)
				continue
			}
			r.Misconfigurations++
			add(f)
		}
		for _, s := range res.Secrets {
			r.Secrets++
			add(Finding{Kind: "secret", Severity: severity(s.Severity), ID: core.Clean(s.RuleID), Title: core.Clean(s.Title), StartLine: s.StartLine, EndLine: s.EndLine})
		}
		for _, l := range res.Licenses {
			f := Finding{Kind: "license", Severity: severity(l.Severity), ID: core.Clean(l.Name), Title: core.Clean(l.Category), Package: core.Clean(l.PkgName), Resource: core.Clean(l.FilePath)}
			r.Licenses++
			if rank(f.Severity) <= rank("HIGH") {
				r.SevereLicenses++
				add(f)
				continue
			}
			hide(f)
		}
		r.Targets = append(r.Targets, t)
	}
	sort.SliceStable(findings, func(i, j int) bool { return less(findings[i], findings[j], true) })
	kept, omitted := core.Limit(findings, o.MaxFindings)
	r.FindingsOmitted = omitted
	for _, f := range findings[len(kept):] {
		r.OmittedBySeverity[f.Severity]++
	}
	sort.SliceStable(kept, func(i, j int) bool { return less(kept[i], kept[j], false) })
	r.Findings = append(r.Findings, kept...)
	sort.SliceStable(r.Hidden, func(i, j int) bool { return less(r.Hidden[i], r.Hidden[j], false) })
	return r
}

func less(a, b Finding, severityFirst bool) bool {
	if severityFirst && rank(a.Severity) != rank(b.Severity) {
		return rank(a.Severity) < rank(b.Severity)
	}
	if a.order != b.order {
		return a.order < b.order
	}
	if a.Kind != b.Kind {
		return a.Kind < b.Kind
	}
	if rank(a.Severity) != rank(b.Severity) {
		return rank(a.Severity) < rank(b.Severity)
	}
	if a.Package != b.Package {
		return a.Package < b.Package
	}
	if a.Resource != b.Resource {
		return a.Resource < b.Resource
	}
	if a.StartLine != b.StartLine {
		return a.StartLine < b.StartLine
	}
	return a.ID < b.ID
}

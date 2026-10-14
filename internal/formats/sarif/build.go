package sarif

import (
	"net/url"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const maxRulesInNotice = 5

type Result struct {
	tfdiag.Diagnostic
	Rule string `json:"rule"`
	Tool string `json:"tool"`
}

type RunSummary struct {
	Tool     string `json:"tool"`
	Version  string `json:"version"`
	Results  int    `json:"results"`
	Errors   int    `json:"errors"`
	Warnings int    `json:"warnings"`
	Notes    int    `json:"notes"`
}

type Report struct {
	Tool           string              `json:"tool"`
	ToolVersion    string              `json:"tool_version"`
	Runs           []RunSummary        `json:"runs"`
	Errors         int                 `json:"errors"`
	Warnings       int                 `json:"warnings"`
	Notes          int                 `json:"notes"`
	Suppressed     int                 `json:"suppressed"`
	OtherKinds     []core.Count        `json:"other_kinds"`
	ToolErrors     int                 `json:"tool_errors"`
	Notifications  []tfdiag.Diagnostic `json:"notifications"`
	Rules          []core.Count        `json:"rules"`
	Results        []Result            `json:"results"`
	ResultsOmitted int                 `json:"results_omitted"`
	OmittedByRule  []core.Count        `json:"omitted_by_rule"`
}

func ruleID(r RawResult, run Run) string {
	if r.RuleID != "" {
		return r.RuleID
	}
	if r.Rule != nil && r.Rule.ID != "" {
		return r.Rule.ID
	}
	if i := r.RuleIndex; i != nil && *i >= 0 && *i < len(run.Tool.Driver.Rules) {
		return run.Tool.Driver.Rules[*i].ID
	}
	return ""
}

func findRule(r RawResult, run Run, id string) *Rule {
	rules := run.Tool.Driver.Rules
	if i := r.RuleIndex; i != nil && *i >= 0 && *i < len(rules) {
		return &rules[*i]
	}
	for i := range rules {
		if rules[i].ID == id {
			return &rules[i]
		}
	}
	return nil
}

func defaultLevel(rule *Rule) string {
	if rule != nil && rule.DefaultConfiguration != nil && rule.DefaultConfiguration.Level != "" {
		return rule.DefaultConfiguration.Level
	}
	return "warning"
}

func severity(level string) string {
	switch level {
	case "warning":
		return "warning"
	case "note":
		return "note"
	default:
		return "error"
	}
}

func suppressed(r RawResult) bool {
	for _, s := range r.Suppressions {
		if s.Status == "" || s.Status == "accepted" {
			return true
		}
	}
	return false
}

func locate(d *tfdiag.Diagnostic, locations []Location) {
	for _, l := range locations {
		p := l.PhysicalLocation
		if p == nil || p.ArtifactLocation == nil || p.ArtifactLocation.URI == "" {
			continue
		}
		uri := strings.TrimPrefix(p.ArtifactLocation.URI, "file://")
		if u, err := url.PathUnescape(uri); err == nil {
			uri = u
		}
		d.File = core.Clean(uri)
		if p.Region != nil {
			d.Line, d.Column = max(p.Region.StartLine, 0), max(p.Region.StartColumn, 0)
		}
		return
	}
}

func split(text string) (string, string) {
	summary, detail, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return core.Clean(summary), strings.TrimSpace(detail)
}

func Build(log *Log, o Options) Report {
	r := Report{Runs: []RunSummary{}, OtherKinds: []core.Count{}, Notifications: []tfdiag.Diagnostic{}, Rules: []core.Count{}, Results: []Result{}, OmittedByRule: []core.Count{}}
	var names []string
	seen := map[string]bool{}
	kinds := map[string]int{}
	counts := map[string]int{}
	var notes []tfdiag.Diagnostic
	var results []Result
	tools := map[string]string{}
	for _, run := range *log.Runs {
		name := core.Clean(run.Tool.Driver.Name)
		version := core.Clean(run.Tool.Driver.Version)
		if version == "" {
			version = core.Clean(run.Tool.Driver.SemanticVersion)
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
		s := RunSummary{Tool: name, Version: version}
		for _, res := range run.Results {
			if res.Kind != "" && res.Kind != "fail" {
				kinds[core.Clean(res.Kind)]++
				continue
			}
			if suppressed(res) {
				r.Suppressed++
				continue
			}
			id := core.Clean(ruleID(res, run))
			rule := findRule(res, run, ruleID(res, run))
			level := res.Level
			if level == "" {
				level = defaultLevel(rule)
			}
			d := tfdiag.Diagnostic{Severity: severity(level), Address: id}
			d.Summary, d.Detail = split(res.Message.Text)
			if d.Detail != "" && rule != nil && rule.ShortDescription != nil && strings.TrimSpace(rule.ShortDescription.Text) != "" {
				d.Summary, d.Detail = core.Clean(rule.ShortDescription.Text), strings.TrimSpace(res.Message.Text)
			}
			if d.Summary == "" {
				d.Summary = "no message"
			}
			locate(&d, res.Locations)
			s.Results++
			switch d.Severity {
			case "error":
				s.Errors++
			case "warning":
				s.Warnings++
			default:
				s.Notes++
			}
			if id != "" {
				counts[id]++
			}
			tools[tfdiag.Key(d)] = name
			results = append(results, Result{Diagnostic: d, Rule: id, Tool: name})
		}
		for _, inv := range run.Invocations {
			if inv.ExecutionSuccessful != nil && !*inv.ExecutionSuccessful {
				r.ToolErrors++
				notes = append(notes, tfdiag.Diagnostic{Severity: "error", Summary: "execution was not successful", Address: name})
			}
			for _, n := range append(append([]Notification(nil), inv.ToolExecutionNotifications...), inv.ToolConfigNotifications...) {
				level := n.Level
				if level == "" {
					level = "warning"
				}
				if level != "error" && level != "warning" {
					continue
				}
				d := tfdiag.Diagnostic{Severity: level, Address: name}
				d.Summary, d.Detail = split(n.Message.Text)
				locate(&d, n.Locations)
				if level == "error" {
					r.ToolErrors++
				}
				notes = append(notes, d)
			}
		}
		r.Errors += s.Errors
		r.Warnings += s.Warnings
		r.Notes += s.Notes
		r.Runs = append(r.Runs, s)
	}
	r.Tool = strings.Join(names, ", ")
	if len(r.Runs) == 1 {
		r.ToolVersion = r.Runs[0].Version
	}
	r.OtherKinds = core.SortedCounts(kinds)
	r.Rules = core.SortedCounts(counts)
	r.Notifications = append(r.Notifications, tfdiag.Normalize(notes)...)
	folded := tfdiag.Normalize(diagnostics(results))
	all := make([]Result, 0, len(folded))
	for _, d := range folded {
		all = append(all, Result{Diagnostic: d, Rule: d.Address, Tool: tools[tfdiag.Key(d)]})
	}
	kept, _ := core.Limit(all, o.MaxResults)
	r.Results = append(r.Results, kept...)
	cut := map[string]int{}
	for _, res := range all[len(kept):] {
		name := res.Rule
		if name == "" {
			name = "no rule"
		}
		cut[name] += max(res.Count, 1)
		r.ResultsOmitted += max(res.Count, 1)
	}
	r.OmittedByRule = core.SortedCounts(cut)
	return r
}

func diagnostics(results []Result) []tfdiag.Diagnostic {
	out := make([]tfdiag.Diagnostic, 0, len(results))
	for _, r := range results {
		out = append(out, r.Diagnostic)
	}
	return out
}

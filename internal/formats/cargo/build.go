package cargo

import (
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type Counts struct {
	Passed      int `json:"passed"`
	Failed      int `json:"failed"`
	Ignored     int `json:"ignored"`
	Measured    int `json:"measured"`
	FilteredOut int `json:"filtered_out"`
}

type Report struct {
	Tool               string       `json:"tool"`
	ToolVersion        string       `json:"tool_version"`
	Status             string       `json:"status"`
	Build              string       `json:"build"`
	FailedUnits        []string     `json:"failed_units"`
	Errors             int          `json:"errors"`
	Warnings           int          `json:"warnings"`
	Fixable            int          `json:"fixable"`
	Codes              []core.Count `json:"codes"`
	Diagnostics        []Diagnostic `json:"diagnostics"`
	DiagnosticsOmitted int          `json:"diagnostics_omitted"`
	OmittedByCode      []core.Count `json:"omitted_by_code"`
	Tests              Counts       `json:"tests"`
	Targets            []Target     `json:"targets"`
	Failures           []Failure    `json:"failures"`
	FailuresOmitted    int          `json:"failures_omitted"`
}

const noCode = "no code"

func Build(run *Run, o Options) Report {
	r := Report{Tool: "cargo", FailedUnits: []string{}, Codes: []core.Count{}, Diagnostics: []Diagnostic{}, OmittedByCode: []core.Count{}, Targets: []Target{}, Failures: []Failure{}}
	r.FailedUnits = append(r.FailedUnits, run.FailedUnits...)
	r.Targets = append(r.Targets, run.Targets...)
	switch {
	case len(run.FailedUnits) > 0 || (run.BuildFinished != nil && !*run.BuildFinished):
		r.Build = "failed"
	case run.BuildFinished != nil || run.Finished || len(run.Targets) > 0:
		r.Build = "ok"
	}
	codes := map[string]int{}
	for _, d := range run.Diagnostics {
		switch d.Severity {
		case "error":
			r.Errors++
		case "warning":
			r.Warnings++
		}
		if d.Fixable {
			r.Fixable++
		}
		if d.ID != "" {
			codes[d.ID]++
		}
	}
	r.Codes = core.SortedCounts(codes)
	ds := fold(run.Diagnostics)
	kept, _ := core.Limit(ds, o.MaxDiagnostics)
	r.Diagnostics = append(r.Diagnostics, kept...)
	cut := map[string]int{}
	for _, d := range ds[len(kept):] {
		name := strings.NewReplacer("(", "", ")", "").Replace(d.ID)
		if name == "" {
			name = noCode
		}
		cut[name] += max(d.Count, 1)
		r.DiagnosticsOmitted += max(d.Count, 1)
	}
	r.OmittedByCode = core.SortedCounts(cut)
	failedTargets := false
	for _, t := range run.Targets {
		r.Tests.Passed += t.Passed
		r.Tests.Failed += t.Failed
		r.Tests.Ignored += t.Ignored
		r.Tests.Measured += t.Measured
		r.Tests.FilteredOut += t.FilteredOut
		failedTargets = failedTargets || failedTarget(t)
	}
	failures, omitted := core.Limit(run.Failures, o.MaxFailures)
	r.Failures = append(r.Failures, failures...)
	r.FailuresOmitted = omitted
	r.Status = "ok"
	if r.Build == "failed" || r.Errors > 0 || r.Tests.Failed > 0 || failedTargets || len(run.Failures) > 0 {
		r.Status = "fail"
	}
	return r
}

func failedTarget(t Target) bool {
	return t.Result == "FAILED" || t.Failed > 0 || (t.Result == "" && t.Rerun != "")
}

func fold(ds []Diagnostic) []Diagnostic {
	plain := make([]tfdiag.Diagnostic, 0, len(ds))
	meta := map[string]Diagnostic{}
	for _, d := range ds {
		plain = append(plain, d.Diagnostic)
		key := tfdiag.Key(d.Diagnostic)
		m := meta[key]
		m.ID = d.ID
		m.Fixable = m.Fixable || d.Fixable
		meta[key] = m
	}
	out := make([]Diagnostic, 0, len(ds))
	for _, d := range tfdiag.Normalize(plain) {
		m := meta[tfdiag.Key(d)]
		out = append(out, Diagnostic{Diagnostic: d, ID: m.ID, Fixable: m.Fixable})
	}
	return out
}

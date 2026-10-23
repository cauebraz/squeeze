package tfjson

import (
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

var policyOutcomes = map[string]string{
	"AllowResult": "pass", "pass": "pass", "n/a": "pass",
	"DenyResult": "fail", "fail": "fail",
	"UnknownResult": "unknown", "unknown": "unknown",
	"InvalidResult": "error", "PolicyErrorResult": "error", "SetupErrorResult": "error", "error": "error",
}

func policyOutcome(result string) string {
	if o, ok := policyOutcomes[result]; ok {
		return o
	}
	return "error"
}

func policyDiagnostic(raw tfdiag.Raw, target string) tfdiag.Diagnostic {
	d := tfdiag.Convert(raw)
	if d.Address == "" {
		d.Address = core.Clean(target)
	}
	return d
}

func (b *builder) policy(name, target, result, enforcement string) bool {
	outcome := policyOutcome(result)
	if outcome == "pass" {
		return false
	}
	b.r.Policies = append(b.r.Policies, PolicyResult{
		Policy: core.Clean(name), Target: core.Clean(target), Result: core.Clean(result),
		Outcome: outcome, Enforcement: core.Clean(enforcement),
	})
	return true
}

func policyInfo(l Line) string {
	parts := []string{"policy info:"}
	if l.PolicyMeta != nil && l.PolicyMeta.PolicyName != "" {
		parts = append(parts, core.Clean(l.PolicyMeta.PolicyName))
	}
	if l.Target != "" {
		parts = append(parts, "for "+core.Clean(l.Target))
	}
	return strings.Join(parts, " ") + ": " + core.Clean(l.PolicyInfo.Message)
}

func (b *builder) querySummary(results []streamQueryResult) {
	for _, r := range results {
		listed := false
		for _, p := range r.Policies {
			for _, raw := range p.Diagnostics {
				b.diagnostic(policyDiagnostic(raw, r.Target), "", "")
			}
			if b.policy(p.Meta.PolicyName, r.Target, p.Result, p.Meta.EnforcementLevel) {
				listed = true
			}
		}
		if !listed {
			b.policy("", r.Target, r.Result, "")
		}
	}
}

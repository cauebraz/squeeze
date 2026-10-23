package tfjson

import (
	"strings"

	"github.com/cauebraz/squeeze/internal/sanitize"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type outcome string

const (
	outcomePass    outcome = "pass"
	outcomeFail    outcome = "fail"
	outcomeUnknown outcome = "unknown"
	outcomeError   outcome = "error"
)

var policyOutcomes = map[string]outcome{
	"AllowResult": outcomePass, "pass": outcomePass, "n/a": outcomePass,
	"DenyResult": outcomeFail, "fail": outcomeFail,
	"UnknownResult": outcomeUnknown, "unknown": outcomeUnknown,
	"InvalidResult": outcomeError, "PolicyErrorResult": outcomeError, "SetupErrorResult": outcomeError, "error": outcomeError,
}

func policyOutcome(result string) outcome {
	if o, ok := policyOutcomes[result]; ok {
		return o
	}
	return outcomeError
}

func policyDiagnostic(raw tfdiag.Raw, target string) tfdiag.Diagnostic {
	d := tfdiag.Convert(raw)
	if d.Address == "" {
		d.Address = sanitize.Clean(target)
	}
	return d
}

func (b *builder) policy(name, target, result, enforcement string) bool {
	o := policyOutcome(result)
	if o == outcomePass {
		return false
	}
	b.r.Policies = append(b.r.Policies, policyResult{
		Policy: sanitize.Clean(name), Target: sanitize.Clean(target), Result: sanitize.Clean(result),
		Outcome: o, Enforcement: sanitize.Clean(enforcement),
	})
	return true
}

func policyInfo(l inputLine) string {
	parts := []string{"policy info:"}
	if l.PolicyMeta != nil && l.PolicyMeta.PolicyName != "" {
		parts = append(parts, sanitize.Clean(l.PolicyMeta.PolicyName))
	}
	if l.Target != "" {
		parts = append(parts, "for "+sanitize.Clean(l.Target))
	}
	return strings.Join(parts, " ") + ": " + sanitize.Clean(l.PolicyInfo.Message)
}

func (b *builder) querySummary(results []rawQueryResult) {
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

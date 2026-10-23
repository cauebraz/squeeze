package trivy

import (
	"bytes"
	"flag"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type options struct {
	maxFindings int
	all         bool
}

func defaultOptions() options {
	return options{maxFindings: 50}
}

type Format struct{}

func (Format) Name() string { return "trivy" }

func (Format) Description() string {
	return "Trivy findings from trivy --format json, including trivy k8s"
}

var conditionNames = []string{"critical", "error", "fail", "fixable", "high", "misconfiguration", "secret", "vulnerability", "warning"}

func (Format) Conditions() []string { return slices.Clone(conditionNames) }

func (Format) Detect(head []byte) bool {
	trimmed := bytes.TrimLeft(head, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false
	}
	if bytes.Contains(head, []byte(`"ClusterName"`)) {
		return bytes.Contains(head, []byte(`"Resources"`)) || bytes.Contains(head, []byte(`"Findings"`))
	}
	if !bytes.Contains(head, []byte(`"SchemaVersion"`)) {
		return false
	}
	return bytes.Contains(head, []byte(`"ArtifactName"`)) || bytes.Contains(head, []byte(`"Results"`))
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.maxFindings, "max-findings", "findings listed, 0 for all")
	fs.BoolVar(&o.all, "all", false, "also list misconfiguration exceptions and licenses below HIGH")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	rep, err := parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := build(rep, o)
	return core.Result{Text: text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r report) []string {
	errs, warns := tfdiag.Count(r.Diagnostics)
	return core.Conditions(map[string]bool{
		"critical":         r.BySeverity["CRITICAL"] > 0,
		"error":            scanErrors(r)+errs > 0,
		"fail":             r.Vulnerabilities+r.Misconfigurations+r.Secrets+r.SevereLicenses > 0,
		"fixable":          r.Fixable > 0,
		"high":             r.BySeverity["HIGH"]+r.BySeverity["CRITICAL"] > 0,
		"misconfiguration": r.Misconfigurations > 0,
		"secret":           r.Secrets > 0,
		"vulnerability":    r.Vulnerabilities > 0,
		"warning":          warns > 0,
	})
}

func scanErrors(r report) int {
	n := 0
	for _, res := range r.Resources {
		if res.Error != "" {
			n++
		}
	}
	return n
}

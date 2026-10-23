package trivy

import (
	"bytes"
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type Options struct {
	MaxFindings int
	All         bool
}

func DefaultOptions() Options {
	return Options{MaxFindings: 50}
}

type Format struct{}

func (Format) Name() string { return "trivy" }

func (Format) Description() string {
	return "Trivy findings from trivy --format json, including trivy k8s"
}

var conditionNames = []string{"critical", "error", "fail", "fixable", "high", "misconfiguration", "secret", "vulnerability", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

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
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxFindings, "max-findings", "findings listed, 0 for all")
	fs.BoolVar(&o.All, "all", false, "also list misconfiguration exceptions and licenses below HIGH")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	rep, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(rep, o)
	return core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	errors, warnings := tfdiag.Count(r.Diagnostics)
	reported := map[string]int{}
	for _, f := range r.Findings {
		reported[f.Severity]++
	}
	for s, n := range r.OmittedBySeverity {
		reported[s] += n
	}
	return core.Conditions(map[string]bool{
		"critical":         reported["CRITICAL"] > 0,
		"error":            scanErrors(r)+errors > 0,
		"fail":             r.Vulnerabilities+r.Misconfigurations+r.Secrets+r.SevereLicenses > 0,
		"fixable":          r.Fixable > 0,
		"high":             reported["HIGH"]+reported["CRITICAL"] > 0,
		"misconfiguration": r.Misconfigurations > 0,
		"secret":           r.Secrets > 0,
		"vulnerability":    r.Vulnerabilities > 0,
		"warning":          warnings > 0,
	})
}

func scanErrors(r Report) int {
	n := 0
	for _, res := range r.Resources {
		if res.Error != "" {
			n++
		}
	}
	return n
}

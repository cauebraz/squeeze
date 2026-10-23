package tfvalidate

import (
	"bytes"
	"flag"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
)

type options struct {
	MaxDiagnostics int
	NoDetail       bool
}

func defaultOptions() options {
	return options{MaxDiagnostics: 20}
}

type Format struct{}

func (Format) Name() string { return "tfvalidate" }

func (Format) Description() string { return "Terraform validation JSON from terraform validate -json" }

var conditionNames = []string{"error", "fail", "invalid", "warning"}

func (Format) Conditions() []string { return slices.Clone(conditionNames) }

func (Format) Detect(head []byte) bool {
	trimmed := bytes.TrimLeft(head, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false
	}
	for _, key := range []string{`"format_version"`, `"valid"`, `"diagnostics"`} {
		if !bytes.Contains(head, []byte(key)) {
			return false
		}
	}
	return !bytes.Contains(head, []byte(`"planned_values"`)) && !bytes.Contains(head, []byte(`"resource_changes"`))
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.MaxDiagnostics, "max-diagnostics", "diagnostics listed, 0 for all")
	fs.BoolVar(&o.NoDetail, "no-detail", false, "print diagnostic summary, location and code only")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	v, err := parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := build(v, o)
	return core.Result{Text: text(r, o), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r report) []string {
	return core.Conditions(map[string]bool{
		"error":   r.Errors > 0,
		"fail":    !r.Valid,
		"invalid": !r.Valid,
		"warning": r.Warnings > 0,
	})
}

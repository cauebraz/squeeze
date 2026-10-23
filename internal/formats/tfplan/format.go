package tfplan

import (
	"bytes"
	"flag"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
)

type options struct {
	MaxAttrs     int
	MaxResources int
	IncludeNoop  bool
	NoValues     bool
	ListDrift    bool
}

func defaultOptions() options {
	return options{MaxAttrs: 8, MaxResources: 200}
}

type Format struct{}

func (Format) Name() string { return "tfplan" }

func (Format) Description() string { return "Terraform plan JSON from terraform show -json" }

var conditionNames = []string{"change", "check", "deferred", "destroy", "drift", "error", "fail", "forget", "import", "incomplete", "move", "replace", "warning"}

func (Format) Conditions() []string { return slices.Clone(conditionNames) }

func (Format) Detect(head []byte) bool {
	trimmed := bytes.TrimLeft(head, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false
	}
	if !bytes.Contains(head, []byte(`"format_version"`)) {
		return false
	}
	return bytes.Contains(head, []byte(`"planned_values"`)) || bytes.Contains(head, []byte(`"resource_changes"`))
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.MaxAttrs, "max-attrs", "attributes per resource listed, 0 for all")
	core.CountFlag(fs, &o.MaxResources, "max-resources", "resources, drifted and deferred resources listed, each, 0 for all")
	fs.BoolVar(&o.IncludeNoop, "all", false, "include resources and outputs without changes, and drift that does not affect the plan")
	fs.BoolVar(&o.NoValues, "no-values", false, "print attribute paths only")
	fs.BoolVar(&o.ListDrift, "drift", false, "list drifted resources instead of counting them")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	p, err := parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := build(p, o)
	return core.Result{Text: text(r, o), Data: r, Conditions: conditions(r)}, nil
}

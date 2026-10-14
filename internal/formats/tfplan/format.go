package tfplan

import (
	"bytes"
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxAttrs     int
	MaxResources int
	IncludeNoop  bool
	NoValues     bool
	ListDrift    bool
}

func DefaultOptions() Options {
	return Options{MaxAttrs: 8, MaxResources: 200}
}

type Format struct{}

func (Format) Name() string { return "tfplan" }

func (Format) Description() string { return "Terraform plan JSON from terraform show -json" }

var conditionNames = []string{"change", "check", "deferred", "destroy", "drift", "error", "fail", "forget", "import", "incomplete", "move", "replace", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

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
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxAttrs, "max-attrs", "attributes per resource listed, 0 for all")
	core.CountFlag(fs, &o.MaxResources, "max-resources", "resources listed, 0 for all")
	fs.BoolVar(&o.IncludeNoop, "all", false, "include resources and outputs without changes, and drift that does not affect the plan")
	fs.BoolVar(&o.NoValues, "no-values", false, "print attribute paths only")
	fs.BoolVar(&o.ListDrift, "drift", false, "list drifted resources instead of counting them")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	p, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r, err := Build(p, o)
	if err != nil {
		return core.Result{}, err
	}
	return core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}, nil
}

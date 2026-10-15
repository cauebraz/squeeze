package tfstate

import (
	"bytes"
	"flag"
	"regexp"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxOutputs   int
	MaxResources int
}

func DefaultOptions() Options {
	return Options{MaxOutputs: 50, MaxResources: 200}
}

type Format struct{}

func (Format) Name() string { return "tfstate" }

func (Format) Description() string {
	return "Terraform state JSON from terraform show -json without a plan, terraform state show -json and terraform output -json"
}

var conditionNames = []string{"error", "fail", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

var showStart = regexp.MustCompile(`^[ \t\r\n]*\{[ \t\r\n]*"format_version"[ \t\r\n]*:[ \t\r\n]*"(?:[^"\\]|\\.)*"[ \t\r\n]*,[ \t\r\n]*"resource"[ \t\r\n]*:[ \t\r\n]*\{`)

var outputsStart = regexp.MustCompile(`^[ \t\r\n]*\{[ \t\r\n]*"(?:[^"\\]|\\.)*"[ \t\r\n]*:[ \t\r\n]*\{[ \t\r\n]*"sensitive"[ \t\r\n]*:[ \t\r\n]*(?:true|false)[ \t\r\n]*,[ \t\r\n]*"type"[ \t\r\n]*:`)

func (Format) Detect(head []byte) bool {
	if outputsStart.Match(head) || showStart.Match(head) {
		return true
	}
	trimmed := bytes.TrimLeft(head, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return false
	}
	for _, key := range []string{`"format_version"`, `"root_module"`} {
		if !bytes.Contains(head, []byte(key)) {
			return false
		}
	}
	return !bytes.Contains(head, []byte(`"planned_values"`)) && !bytes.Contains(head, []byte(`"resource_changes"`))
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxOutputs, "max-outputs", "outputs listed, 0 for all")
	core.CountFlag(fs, &o.MaxResources, "max-resources", "state resources listed, 0 for all")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	in, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(in, o)
	return core.Result{Text: Text(r), Data: r, Conditions: core.Conditions(map[string]bool{
		"error":   r.Errors > 0,
		"fail":    r.Errors > 0,
		"warning": r.Warnings > 0,
	})}, nil
}

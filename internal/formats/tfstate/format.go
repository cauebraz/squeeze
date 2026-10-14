package tfstate

import (
	"flag"
	"regexp"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxOutputs int
}

func DefaultOptions() Options {
	return Options{MaxOutputs: 50}
}

type Format struct{}

func (Format) Name() string { return "tfstate" }

func (Format) Description() string { return "Terraform outputs JSON from terraform output -json" }

func (Format) Conditions() []string { return []string{} }

var outputsStart = regexp.MustCompile(`^[ \t\r\n]*\{[ \t\r\n]*"(?:[^"\\]|\\.)*"[ \t\r\n]*:[ \t\r\n]*\{[ \t\r\n]*"sensitive"[ \t\r\n]*:[ \t\r\n]*(?:true|false)[ \t\r\n]*,[ \t\r\n]*"type"[ \t\r\n]*:`)

func (Format) Detect(head []byte) bool {
	return outputsStart.Match(head)
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxOutputs, "max-outputs", "outputs listed, 0 for all")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	outputs, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(outputs, o)
	return core.Result{Text: Text(r), Data: r, Conditions: core.Conditions(nil)}, nil
}

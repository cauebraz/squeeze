package sarif

import (
	"bytes"
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxResults int
	NoDetail   bool
}

func DefaultOptions() Options {
	return Options{MaxResults: 50}
}

type Format struct{}

func (Format) Name() string { return "sarif" }

func (Format) Description() string {
	return "SARIF 2.1.0 results from trivy, tflint, checkov, semgrep and other tools"
}

var conditionNames = []string{"error", "fail", "note", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

func (Format) Detect(head []byte) bool {
	trimmed := bytes.TrimLeft(head, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' || !bytes.Contains(head, []byte(`"runs"`)) {
		return false
	}
	return bytes.Contains(head, []byte("sarif-schema-2.1.0")) || bytes.Contains(head, []byte(`"version":"2.1.0"`)) || bytes.Contains(head, []byte(`"version": "2.1.0"`))
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxResults, "max-results", "results listed, 0 for all")
	fs.BoolVar(&o.NoDetail, "no-detail", false, "print the first line of each message, its location and rule only")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	log, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(log, o)
	return core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	failed := r.Errors > 0 || r.Warnings > 0 || r.ToolErrors > 0
	return core.Conditions(map[string]bool{
		"error":   r.Errors > 0 || r.ToolErrors > 0,
		"fail":    failed,
		"note":    r.Notes > 0,
		"warning": r.Warnings > 0,
	})
}

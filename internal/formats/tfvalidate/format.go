package tfvalidate

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"strconv"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxDiagnostics int
	NoDetail       bool
}

func DefaultOptions() Options {
	return Options{MaxDiagnostics: 20}
}

type Format struct{}

func (Format) Name() string { return "tfvalidate" }

func (Format) Description() string { return "Terraform validation JSON from terraform validate -json" }

var conditionNames = []string{"error", "invalid", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

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
	o := DefaultOptions()
	fs.Func("max-diagnostics", fmt.Sprintf("diagnostics shown, 0 for all (default %d)", o.MaxDiagnostics), func(s string) error {
		n, err := strconv.Atoi(s)
		if err != nil {
			return errors.New("not an integer")
		}
		if n < 0 {
			return errors.New("must be 0 or more")
		}
		o.MaxDiagnostics = n
		return nil
	})
	fs.BoolVar(&o.NoDetail, "no-detail", false, "print summary, location and code only")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	v, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(v, o)
	return core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	return core.Conditions(map[string]bool{
		"error":   r.Errors > 0,
		"invalid": !r.Valid,
		"warning": r.Warnings > 0,
	})
}

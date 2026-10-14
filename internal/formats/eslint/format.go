package eslint

import (
	"bytes"
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxProblems int
}

func DefaultOptions() Options {
	return Options{MaxProblems: 50}
}

type Format struct{}

func (Format) Name() string { return "eslint" }

func (Format) Description() string {
	return "ESLint problems from eslint -f json or -f json-with-metadata"
}

var conditionNames = []string{"error", "fail", "fixable", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

var prefixes = [][]byte{[]byte(`[{"filePath":`), []byte(`{"results":[{"filePath":`)}

func (Format) Detect(head []byte) bool {
	var start []byte
	for _, c := range head {
		if len(start) == len(prefixes[1]) {
			break
		}
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			start = append(start, c)
		}
	}
	for _, p := range prefixes {
		if bytes.HasPrefix(start, p) {
			return bytes.Contains(head, []byte(`"messages"`)) && bytes.Contains(head, []byte(`"errorCount"`))
		}
	}
	return false
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxProblems, "max-problems", "problems listed, 0 for all")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	out, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(out, o)
	return core.Result{Text: Text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	return core.Conditions(map[string]bool{
		"error":   r.Errors > 0,
		"fail":    r.Errors > 0,
		"fixable": r.Fixable > 0,
		"warning": r.Warnings > 0,
	})
}

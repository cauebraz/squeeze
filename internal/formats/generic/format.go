package generic

import (
	"bytes"
	"flag"
	"slices"
	"unicode/utf8"

	"github.com/cauebraz/squeeze/internal/core"
)

type options struct {
	MaxLines     int
	MaxLineBytes int
	NoFold       bool
}

func defaultOptions() options {
	return options{MaxLines: 200, MaxLineBytes: 512}
}

type Format struct{}

func (Format) Name() string { return "generic" }

func (Format) Description() string {
	return "any text output: strips color codes, folds repeated lines, keeps the head, the tail and error lines"
}

var conditionNames = []string{"error", "warning"}

func (Format) Conditions() []string { return slices.Clone(conditionNames) }

func (Format) Detect(head []byte) bool {
	if len(bytes.TrimSpace(head)) == 0 {
		return len(head) == core.HeadSize
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return false
	}
	return utf8.Valid(trimPartialRune(head))
}

func trimPartialRune(b []byte) []byte {
	for i := 1; i < utf8.UTFMax && i <= len(b); i++ {
		if c := b[len(b)-i]; c&0xC0 != 0x80 {
			if c >= 0xC0 && !utf8.FullRune(b[len(b)-i:]) {
				return b[:len(b)-i]
			}
			break
		}
	}
	return b
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.MaxLines, "max-lines", "lines listed, 0 for all")
	core.CountFlag(fs, &o.MaxLineBytes, "max-line-bytes", "bytes per line, 0 for all")
	fs.BoolVar(&o.NoFold, "no-fold", false, "print repeated and similar lines")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	r := build(parse(input, o.MaxLineBytes), o)
	return core.Result{Text: text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r report) []string {
	return core.Conditions(map[string]bool{
		"error":   r.ErrorLines > 0,
		"warning": r.WarningLines > 0,
	})
}

package generic

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxLines     int
	MaxLineBytes int
	NoFold       bool
}

func DefaultOptions() Options {
	return Options{MaxLines: 200, MaxLineBytes: 512}
}

type Format struct{}

func (Format) Name() string { return "generic" }

func (Format) Description() string {
	return "any text output: strips color codes, folds repeated lines, keeps the head, the tail and error lines"
}

func (Format) Conditions() []string { return []string{} }

func (Format) Detect(head []byte) bool {
	if len(bytes.TrimSpace(head)) == 0 || bytes.IndexByte(head, 0) >= 0 {
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
	o := DefaultOptions()
	countFlag(fs, &o.MaxLines, "max-lines", "lines shown, 0 for all")
	countFlag(fs, &o.MaxLineBytes, "max-line-bytes", "bytes shown per line, 0 for all")
	fs.BoolVar(&o.NoFold, "no-fold", false, "print repeated and similar lines instead of folding them")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	r, text := Build(input, o)
	return core.Result{Text: text, Data: r, Conditions: []string{}}, nil
}

func countFlag(fs *flag.FlagSet, p *int, name, usage string) {
	fs.Func(name, fmt.Sprintf("%s (default %d)", usage, *p), func(s string) error {
		n, err := strconv.Atoi(s)
		if err != nil {
			return errors.New("not an integer")
		}
		if n < 0 {
			return errors.New("must be 0 or more")
		}
		*p = n
		return nil
	})
}

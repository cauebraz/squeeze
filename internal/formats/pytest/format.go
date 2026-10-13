package pytest

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"strconv"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxTests    int
	MaxOutput   int
	MaxWarnings int
	Full        bool
	All         bool
}

func DefaultOptions() Options {
	return Options{MaxTests: 50, MaxOutput: 20, MaxWarnings: 10}
}

type Format struct{}

func (Format) Name() string { return "pytest" }

func (Format) Description() string { return "pytest terminal output, default, -v or -q" }

var conditionNames = []string{"error", "fail", "interrupted", "no_tests", "skip", "warning", "xfail", "xpass"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

func (Format) Detect(head []byte) bool {
	for _, raw := range bytes.Split(head, []byte("\n")) {
		l := core.Line(string(raw))
		if sessionStart.MatchString(l) || knownSection.MatchString(l) || statsLine.MatchString(l) {
			return true
		}
	}
	return false
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	countFlag(fs, &o.MaxTests, "max-tests", "failures and errors listed, 0 for all")
	countFlag(fs, &o.MaxOutput, "max-output", "lines per failure, 0 for all")
	countFlag(fs, &o.MaxWarnings, "max-warnings", "warnings listed, 0 for all")
	fs.BoolVar(&o.Full, "full", false, "print the whole traceback and the captured output of every failure")
	fs.BoolVar(&o.All, "all", false, "also print skip, xfail and xpass lines and the sections squeeze only counts")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

var ErrNoReport = errors.New("no pytest session, section or summary line in the input")

func Squeeze(input []byte, o Options) (core.Result, error) {
	r, ok := Build(input, o)
	if !ok {
		return core.Result{}, ErrNoReport
	}
	return core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	s := r.Summary
	return core.Conditions(map[string]bool{
		"error":       s.Errors > 0 || r.Interrupted != "",
		"fail":        s.Failed > 0 || s.Errors > 0 || r.Interrupted != "" || len(r.Failures)+r.FailuresOmitted > 0,
		"interrupted": r.Interrupted != "",
		"no_tests":    r.Complete && s.Failed+s.Passed+s.Skipped+s.XFailed+s.XPassed+s.Errors == 0,
		"skip":        s.Skipped > 0,
		"warning":     s.Warnings > 0 || len(r.Warnings)+r.WarningsOmitted > 0,
		"xfail":       s.XFailed > 0,
		"xpass":       s.XPassed > 0,
	})
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

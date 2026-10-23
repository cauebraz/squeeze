package pytest

import (
	"bytes"
	"errors"
	"flag"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/sanitize"
)

type options struct {
	MaxTests    int
	MaxOutput   int
	MaxWarnings int
	Full        bool
	All         bool
	NoFold      bool
}

func defaultOptions() options {
	return options{MaxTests: 50, MaxOutput: 20, MaxWarnings: 10}
}

type Format struct{}

func (Format) Name() string { return "pytest" }

func (Format) Description() string { return "pytest terminal output, default, -v or -q" }

var conditionNames = []string{"error", "fail", "internal_error", "interrupted", "no_tests", "skip", "warning", "xfail", "xpass"}

func (Format) Conditions() []string { return slices.Clone(conditionNames) }

func (Format) Detect(head []byte) bool {
	for _, raw := range bytes.Split(head, []byte("\n")) {
		l := sanitize.Line(string(raw))
		if sessionStart.MatchString(l) || knownSection.MatchString(l) {
			return true
		}
		if m := statsLine.FindStringSubmatch(l); m != nil && knownStats(m[1]) {
			return true
		}
	}
	return false
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.MaxTests, "max-tests", "failed or errored tests listed, 0 for all")
	core.CountFlag(fs, &o.MaxOutput, "max-output", "lines per failure and lines outside the sections, 0 for all")
	core.CountFlag(fs, &o.MaxWarnings, "max-warnings", "warnings listed, 0 for all")
	fs.BoolVar(&o.Full, "full", false, "print the whole traceback and the captured output of every failure")
	fs.BoolVar(&o.NoFold, "no-fold", false, "print every failure, also one that fails at the same line as an earlier one")
	fs.BoolVar(&o.All, "all", false, "print the short summary lines of passed, skipped, xfailed and xpassed tests and the sections squeeze only counts")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

var errNotPytest = errors.New("no pytest session, section or summary line in the input")

func squeeze(input []byte, o options) (core.Result, error) {
	r, ok := build(input, o)
	if !ok {
		return core.Result{}, errNotPytest
	}
	return core.Result{Text: render(r, o), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r report) []string {
	s := r.Summary
	return core.Conditions(map[string]bool{
		"error":          s.Errors > 0 || r.Interrupted != "",
		"fail":           s.Failed > 0 || s.Errors > 0 || r.Interrupted != "" || r.InternalError || len(r.Failures)+r.TestsOmitted > 0,
		"internal_error": r.InternalError,
		"interrupted":    r.Interrupted != "",
		"no_tests":       r.SummaryFound && s.Failed+s.Passed+s.Skipped+s.XFailed+s.XPassed+s.Errors == 0,
		"skip":           s.Skipped > 0,
		"warning":        s.Warnings > 0 || len(r.WarningMessages)+r.WarningsOmitted > 0,
		"xfail":          s.XFailed > 0,
		"xpass":          s.XPassed > 0,
	})
}

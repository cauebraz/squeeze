package cargo

import (
	"bytes"
	"flag"
	"regexp"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxDiagnostics int
	MaxFailures    int
	MaxOutput      int
}

func DefaultOptions() Options {
	return Options{MaxDiagnostics: 50, MaxFailures: 20, MaxOutput: 10}
}

type Format struct{}

func (Format) Name() string { return "cargo" }

func (Format) Description() string {
	return "Rust builds and tests from cargo --message-format=json or cargo test text"
}

var conditionNames = []string{"build_failed", "error", "fail", "fixable", "ignored", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

var (
	reasons    = []string{"compiler-message", "compiler-artifact", "build-script-executed", "build-finished"}
	finishedRe = regexp.MustCompile("^ +Finished (`[\\w-]+` profile|[\\w-]+) \\[[^\\]]*\\] target\\(s\\) in ")
)

func (Format) Detect(head []byte) bool {
	running, tests := false, false
	for _, raw := range bytes.Split(head, []byte("\n")) {
		line := string(bytes.TrimRight(raw, "\r"))
		switch {
		case isMessage(line), couldNotCompileRe.MatchString(line), finishedRe.MatchString(line):
			return true
		case runningRe.MatchString(line):
			running = true
		case resultRe.MatchString(line), testLineRe.MatchString(line):
			tests = true
		}
	}
	return running && tests
}

func isMessage(line string) bool {
	for _, r := range reasons {
		if strings.HasPrefix(line, `{"reason":"`+r+`"`) {
			return true
		}
	}
	return false
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxDiagnostics, "max-diagnostics", "compiler diagnostics listed, 0 for all")
	core.CountFlag(fs, &o.MaxFailures, "max-failures", "failed tests listed, 0 for all")
	core.CountFlag(fs, &o.MaxOutput, "max-output", "panic and output lines per failed test, 0 for all")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	run, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(run, o)
	return core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	return core.Conditions(map[string]bool{
		"build_failed": r.Build == "failed",
		"error":        r.Errors > 0,
		"fail":         r.Status == "fail",
		"fixable":      r.Fixable > 0,
		"ignored":      r.Tests.Ignored > 0,
		"warning":      r.Warnings > 0,
	})
}

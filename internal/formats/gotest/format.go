package gotest

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"strconv"

	"github.com/cauebraz/squeeze/internal/core"
)

const detectLines = 5

type Options struct {
	MaxTests  int
	MaxOutput int
	Logs      bool
	All       bool
}

func DefaultOptions() Options {
	return Options{MaxTests: 50, MaxOutput: 20}
}

type Format struct{}

func (Format) Name() string { return "gotest" }

func (Format) Description() string { return "Go test results from go test -json" }

var conditionNames = []string{"build_failed", "fail", "panic", "race", "skip", "timeout"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

func (Format) Detect(head []byte) bool {
	seen := 0
	for _, line := range bytes.Split(head, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if isEvent(line) {
			return true
		}
		seen++
		if seen == detectLines {
			return false
		}
	}
	return false
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	countFlag(fs, &o.MaxTests, "max-tests", "failed tests listed, 0 for all")
	countFlag(fs, &o.MaxOutput, "max-output", "output lines per failure, 0 for all")
	fs.BoolVar(&o.Logs, "logs", false, "print every output line of a failure: logs and the full stack")
	fs.BoolVar(&o.All, "all", false, "list skipped tests, passing packages and packages without tests")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	events, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(events, o)
	return core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	return core.Conditions(map[string]bool{
		"build_failed": r.Packages.BuildFailed > 0,
		"fail":         r.Status == "fail",
		"panic":        r.Panics > 0,
		"race":         r.Races > 0,
		"skip":         r.Tests.Skipped > 0,
		"timeout":      r.Timeouts > 0,
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

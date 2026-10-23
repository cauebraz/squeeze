package gotest

import (
	"bytes"
	"errors"
	"flag"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/jsonl"
)

const detectLines = 20

type options struct {
	MaxTests  int
	MaxOutput int
	Full      bool
	All       bool
}

func defaultOptions() options {
	return options{MaxTests: 50, MaxOutput: 20}
}

type Format struct{}

func (Format) Name() string { return "gotest" }

func (Format) Description() string {
	return "Go test results from go test -json or go test text output"
}

var conditionNames = []string{"build_failed", "error", "fail", "panic", "race", "skip", "timeout", "warning"}

func (Format) Conditions() []string { return slices.Clone(conditionNames) }

func (Format) Detect(head []byte) bool {
	return jsonl.Detect(head, detectLines, func(l []byte) bool {
		return bytes.HasPrefix(l, []byte("go: ")) || bytes.HasPrefix(l, []byte("# "))
	}, isEvent) || detectPlain(head)
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.MaxTests, "max-tests", "failed tests listed, 0 for all")
	core.CountFlag(fs, &o.MaxOutput, "max-output", "output lines per failure, build failure or package, 0 for all")
	fs.BoolVar(&o.Full, "full", false, "print every output line of a failure: logs and the full stack")
	fs.BoolVar(&o.All, "all", false, "also list skipped tests, passing packages and packages without tests")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	events, plain, err := parseInput(input)
	if err != nil {
		return core.Result{}, err
	}
	r := build(events, o, plain)
	return core.Result{Text: render(r, o), Data: r, Conditions: conditions(r)}, nil
}

func parseInput(input []byte) ([]rawEvent, bool, error) {
	events, err := parseEvents(input)
	if !errors.Is(err, errNotGoTestJSON) {
		return events, false, err
	}
	events, err = parsePlain(input)
	return events, true, err
}

func conditions(r report) []string {
	return core.Conditions(map[string]bool{
		"build_failed": r.Packages.BuildFailed > 0,
		"error":        r.Packages.BuildFailed > 0 || r.Panics+r.Timeouts+r.Races > 0 || unfinished(r),
		"fail":         r.Status == statusFail,
		"panic":        r.Panics > 0,
		"race":         r.Races > 0,
		"skip":         r.Tests.Skipped > 0,
		"timeout":      r.Timeouts > 0,
		"warning":      len(r.WarningMessages) > 0,
	})
}

func unfinished(r report) bool {
	for _, p := range r.PackageFailures {
		if p.Unfinished {
			return true
		}
	}
	for _, f := range r.Failures {
		if f.Unfinished {
			return true
		}
	}
	return false
}

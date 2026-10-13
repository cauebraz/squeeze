package gotest

import (
	"bytes"
	"flag"
	"github.com/cauebraz/squeeze/internal/jsonl"

	"github.com/cauebraz/squeeze/internal/core"
)

const detectLines = 20

type Options struct {
	MaxTests  int
	MaxOutput int
	Full      bool
	All       bool
}

func DefaultOptions() Options {
	return Options{MaxTests: 50, MaxOutput: 20}
}

type Format struct{}

func (Format) Name() string { return "gotest" }

func (Format) Description() string { return "Go test results from go test -json" }

var conditionNames = []string{"build_failed", "error", "fail", "panic", "race", "skip", "timeout", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

func (Format) Detect(head []byte) bool {
	return jsonl.Detect(head, detectLines, func(line []byte) bool {
		return bytes.HasPrefix(line, []byte("go: ")) || bytes.HasPrefix(line, []byte("# "))
	}, isEvent)
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxTests, "max-tests", "failed tests listed, 0 for all")
	core.CountFlag(fs, &o.MaxOutput, "max-output", "output lines per failure, build failure or package, 0 for all")
	fs.BoolVar(&o.Full, "full", false, "print every output line of a failure: logs and the full stack")
	fs.BoolVar(&o.All, "all", false, "also list skipped tests, passing packages and packages without tests")
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
		"error":        r.Packages.BuildFailed > 0 || r.Panics+r.Timeouts+r.Races > 0 || unfinished(r),
		"fail":         r.Status == "fail",
		"panic":        r.Panics > 0,
		"race":         r.Races > 0,
		"skip":         r.Tests.Skipped > 0,
		"timeout":      r.Timeouts > 0,
		"warning":      len(r.Warnings) > 0,
	})
}

func unfinished(r Report) bool {
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

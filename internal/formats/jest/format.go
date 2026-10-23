package jest

import (
	"bytes"
	"flag"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
)

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

func (Format) Name() string { return "jest" }

func (Format) Description() string {
	return "Jest and Vitest results from jest --json or vitest --reporter=json"
}

var conditionNames = []string{"error", "fail", "skip", "snapshot", "todo", "warning"}

func (Format) Conditions() []string { return slices.Clone(conditionNames) }

func (Format) Detect(head []byte) bool {
	trimmed := bytes.TrimLeft(head, " \t\r\n")
	if !bytes.HasPrefix(trimmed, []byte("{")) {
		return false
	}
	trimmed = bytes.TrimLeft(trimmed[1:], " \t\r\n")
	if !bytes.HasPrefix(trimmed, []byte(`"num`)) {
		return false
	}
	for _, key := range []string{`"numTotalTests"`, `"numFailedTestSuites"`, `"testResults"`} {
		if !bytes.Contains(head, []byte(key)) {
			return false
		}
	}
	return true
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.MaxTests, "max-tests", "failed tests listed, 0 for all")
	core.CountFlag(fs, &o.MaxOutput, "max-output", "message lines per failed test, file error or open handle, 0 for all")
	fs.BoolVar(&o.Full, "full", false, "keep node_modules and node internal stack frames and code frame context")
	fs.BoolVar(&o.All, "all", false, "also list skipped and todo tests and passing files")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	out, err := parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := build(out, o)
	return core.Result{Text: render(r, o), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r report) []string {
	return core.Conditions(map[string]bool{
		"error":    len(r.FileErrors) > 0 || r.RunError != nil,
		"fail":     r.Status == statusFail,
		"skip":     r.Tests.Skipped > 0,
		"snapshot": r.Snapshots.Failed > 0 || r.Snapshots.Failure,
		"todo":     r.Tests.Todo > 0,
		"warning":  len(r.OpenHandles) > 0,
	})
}

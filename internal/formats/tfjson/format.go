package tfjson

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
	MaxResources   int
	MaxDiagnostics int
	All            bool
	ListDrift      bool
	NoDetail       bool
}

func DefaultOptions() Options {
	return Options{MaxResources: 200, MaxDiagnostics: 20}
}

type Format struct{}

func (Format) Name() string { return "tfjson" }

func (Format) Description() string {
	return "Terraform machine-readable UI stream from plan, apply, destroy, init or test with -json"
}

var conditionNames = []string{"apply_errored", "change", "deferred", "destroy", "drift", "error", "forget", "policy_failed", "replace", "test_failed", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

func (Format) Detect(head []byte) bool {
	seen := 0
	for _, line := range bytes.Split(head, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if isStreamLine(line) {
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
	countFlag(fs, &o.MaxResources, "max-resources", "resources shown, 0 for all")
	countFlag(fs, &o.MaxDiagnostics, "max-diagnostics", "diagnostics shown, 0 for all")
	fs.BoolVar(&o.All, "all", false, "include no-op resources, passing tests and every other message")
	fs.BoolVar(&o.ListDrift, "drift", false, "list drifted resources instead of counting them")
	fs.BoolVar(&o.NoDetail, "no-detail", false, "print diagnostic summary, location and code only")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	lines, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(lines, o)
	return core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r, lines)}, nil
}

func conditions(r Report, lines []Line) []string {
	present := map[string]bool{
		"apply_errored": len(r.Failures) > 0,
		"deferred":      len(r.Deferred) > 0,
		"drift":         len(r.Drift) > 0,
		"error":         r.Errors > 0 || len(r.Failures) > 0,
		"policy_failed": len(r.Policies) > 0,
		"warning":       r.Warnings > 0,
	}
	if s := r.Summary; s != nil {
		present["change"] = s.Add+s.Change+s.Remove+s.Import > 0
		present["destroy"] = s.Remove > 0
	}
	for _, l := range lines {
		if l.Type != "planned_change" || l.Change == nil {
			continue
		}
		switch l.Change.Action {
		case "delete":
			present["destroy"], present["change"] = true, true
		case "replace":
			present["destroy"], present["replace"], present["change"] = true, true, true
		case "remove":
			present["forget"], present["change"] = true, true
		case "create", "update", "import", "move":
			present["change"] = true
		}
	}
	if t := r.Tests; t != nil {
		present["test_failed"] = t.Failed+t.Errored > 0
		for _, run := range t.Runs {
			if failingTestStatus[run.Status] {
				present["test_failed"] = true
			}
		}
		for _, f := range t.Files {
			if failingTestStatus[f.Status] {
				present["test_failed"] = true
			}
		}
	}
	return core.Conditions(present)
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

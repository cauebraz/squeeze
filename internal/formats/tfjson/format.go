package tfjson

import (
	"flag"
	"slices"

	"github.com/cauebraz/squeeze/internal/jsonl"

	"github.com/cauebraz/squeeze/internal/core"
)

const detectLines = 5

type options struct {
	MaxResources   int
	MaxDiagnostics int
	All            bool
	ListDrift      bool
	NoDetail       bool
}

func defaultOptions() options {
	return options{MaxResources: 200, MaxDiagnostics: 20}
}

type Format struct{}

func (Format) Name() string { return "tfjson" }

func (Format) Description() string {
	return "Terraform machine-readable UI stream from plan, apply, destroy, init, test or query with -json"
}

var conditionNames = []string{"apply_errored", "change", "deferred", "destroy", "drift", "error", "fail", "forget", "policy_failed", "replace", "test_failed", "warning"}

func (Format) Conditions() []string { return slices.Clone(conditionNames) }

func (Format) Detect(head []byte) bool {
	return jsonl.Detect(head, detectLines, nil, isStreamLine)
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.MaxResources, "max-resources", "planned changes, found, drifted and deferred resources listed, each, 0 for all")
	core.CountFlag(fs, &o.MaxDiagnostics, "max-diagnostics", "diagnostics listed, 0 for all")
	fs.BoolVar(&o.All, "all", false, "include no-op resources, passing tests and every other message")
	fs.BoolVar(&o.ListDrift, "drift", false, "list drifted resources instead of counting them")
	fs.BoolVar(&o.NoDetail, "no-detail", false, "print diagnostic summary, location and code only")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	lines, err := parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := build(lines, o)
	return core.Result{Text: text(r, o), Data: r, Conditions: conditions(r, lines)}, nil
}

func conditions(r report, lines []inputLine) []string {
	present := map[string]bool{
		"apply_errored": len(r.Failures) > 0,
		"deferred":      len(r.Deferred) > 0,
		"drift":         len(r.Drift)+r.DriftOmitted > 0,
		"error":         r.Errors > 0 || len(r.Failures) > 0 || hasPolicyOutcome(r, outcomeError),
		"policy_failed": hasPolicyOutcome(r, outcomeFail),
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
	present["fail"] = present["error"] || present["apply_errored"] || present["policy_failed"] || present["test_failed"]
	return core.Conditions(present)
}

func hasPolicyOutcome(r report, o outcome) bool {
	for _, p := range r.Policies {
		if p.Outcome == o {
			return true
		}
	}
	return false
}

package kubectl

import (
	"bytes"
	"flag"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/logscan"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type options struct {
	maxObjects int
	maxEvents  int
}

func defaultOptions() options {
	return options{maxObjects: 50, maxEvents: 20}
}

type Format struct{}

func (Format) Name() string { return "kubectl" }

func (Format) Description() string {
	return "Kubernetes objects and events from kubectl get -o json"
}

var conditionNames = []string{"error", "fail", "restarts", "warning"}

func (Format) Conditions() []string { return slices.Clone(conditionNames) }

var prefix = []byte(`{"apiVersion":`)

func (Format) Detect(head []byte) bool {
	_, rest := logscan.LeadingLines(head)
	return core.HasJSONPrefix(rest, prefix) && bytes.Contains(rest, []byte(`"kind":`)) && bytes.Contains(rest, []byte(`"metadata":`))
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := defaultOptions()
	core.CountFlag(fs, &o.maxObjects, "max-objects", "objects listed, unhealthy first, 0 for all")
	core.CountFlag(fs, &o.maxEvents, "max-events", "warning events listed, 0 for all")
	return func(input []byte) (core.Result, error) {
		return squeeze(input, o)
	}
}

func squeeze(input []byte, o options) (core.Result, error) {
	in, err := parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := build(in, o)
	return core.Result{Text: text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r report) []string {
	errs, warns := tfdiag.Count(r.Diagnostics)
	return core.Conditions(map[string]bool{
		"error":    errs > 0,
		"fail":     r.Unhealthy > 0,
		"restarts": r.Restarts > 0,
		"warning":  r.Warnings+warns > 0,
	})
}

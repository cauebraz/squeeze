package kubectl

import (
	"bytes"
	"flag"

	"github.com/cauebraz/squeeze/internal/core"
)

type Options struct {
	MaxObjects int
	MaxEvents  int
}

func DefaultOptions() Options {
	return Options{MaxObjects: 50, MaxEvents: 20}
}

type Format struct{}

func (Format) Name() string { return "kubectl" }

func (Format) Description() string {
	return "Kubernetes objects and events from kubectl get -o json"
}

var conditionNames = []string{"fail", "restarts", "warning"}

func (Format) Conditions() []string { return append([]string(nil), conditionNames...) }

var prefix = []byte(`{"apiVersion":`)

func (Format) Detect(head []byte) bool {
	var start []byte
	for _, c := range head {
		if len(start) == len(prefix) {
			break
		}
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			start = append(start, c)
		}
	}
	return bytes.Equal(start, prefix) && bytes.Contains(head, []byte(`"kind":`)) && bytes.Contains(head, []byte(`"metadata":`))
}

func (Format) Configure(fs *flag.FlagSet) core.Squeezer {
	o := DefaultOptions()
	core.CountFlag(fs, &o.MaxObjects, "max-objects", "objects listed, unhealthy first, 0 for all")
	core.CountFlag(fs, &o.MaxEvents, "max-events", "warning events listed, 0 for all")
	return func(input []byte) (core.Result, error) {
		return Squeeze(input, o)
	}
}

func Squeeze(input []byte, o Options) (core.Result, error) {
	objs, err := Parse(input)
	if err != nil {
		return core.Result{}, err
	}
	r := Build(objs, o)
	return core.Result{Text: Text(r), Data: r, Conditions: conditions(r)}, nil
}

func conditions(r Report) []string {
	return core.Conditions(map[string]bool{
		"fail":     r.Unhealthy > 0,
		"restarts": r.Restarts > 0,
		"warning":  r.Warnings > 0,
	})
}

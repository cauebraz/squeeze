package tfplan

import (
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
)

func FuzzSqueeze(f *testing.F) {
	for _, data := range fixtures(f) {
		f.Add(data)
	}
	f.Add([]byte(`{"format_version":"1.2","resource_changes":[{"address":"a","change":{"actions":["update"],"before":{"a":[1,{"b":null}]},"after":{"a":[2]},"after_unknown":{"a":[false,true]},"before_sensitive":{"a":[true]},"replace_paths":[["a",0]]}}]}`))
	f.Add([]byte(`{"format_version":"1.2","resource_changes":[{"address":"a","change":{"actions":["update"],"before":1,"after":"x","after_unknown":[true],"before_sensitive":{"x":true}}}]}`))
	f.Add([]byte(`{"format_version":"1.2","resource_drift":[{"address":"a","change":{"actions":["update"],"before":{"t":{"k":"s3cret"}},"after":{"t":{}},"before_sensitive":{"t":{"k":true}}}}],"relevant_attributes":[{"resource":"","attribute":["t"]}]}`))
	f.Fuzz(func(t *testing.T, input []byte) {
		p, err := Parse(input)
		if err != nil {
			return
		}
		for _, o := range []Options{DefaultOptions(), {IncludeNoop: true, ListDrift: true}} {
			r, err := Build(p, o)
			if err != nil {
				return
			}
			res := core.Result{Text: Text(r, o), Data: r}
			formattest.CheckText(t, res.Text)
			if _, err := core.Envelope("tfplan", len(input), res); err != nil {
				t.Fatalf("Envelope() error after successful Build: %v", err)
			}
		}
		for _, list := range [][]PlanResource{p.ResourceChanges, p.ResourceDrift} {
			for _, rc := range list {
				attrs, _ := diffAttrs(rc.Change, true)
				for _, a := range attrs {
					if a.Before != nil && markedAt(rc.Change.BeforeSensitive, a.at) {
						t.Fatalf("%s %s: before value of a sensitive path kept in the model", rc.Address, a.Path)
					}
					if a.After != nil && markedAt(rc.Change.AfterSensitive, a.at) {
						t.Fatalf("%s %s: after value of a sensitive path kept in the model", rc.Address, a.Path)
					}
				}
			}
		}
	})
}

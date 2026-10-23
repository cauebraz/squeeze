package tfplan

import (
	"slices"
	"strings"
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
	f.Add([]byte(`{"format_version":"1.2","resource_changes":[{"address":"a","change":{"actions":["update"],"before":{"l":[{"k":"s3cret"},{"k":"x"}]},"after":{"l":[{"k":"x"},{"k":"t"}]},"after_unknown":{},"before_sensitive":{"l":[{"k":true},{}]},"after_sensitive":{"l":[{},{}]}}}]}`))
	f.Add([]byte(`{"format_version":"1.2","resource_changes":[{"address":"a","change":{"actions":["update"],"before":{"pw":"s3cret","l":["x","s3cret"]},"after":{"pw":"s3cret2","l":["x","s3cret2"]},"before_sensitive":{"pw":1,"l":[false]},"after_sensitive":{"pw":"true","l":[false]}}}]}`))
	f.Add([]byte(`{"format_version":"1.2","deferred_action_invocations":[{"reason":"deferred_prereq","action_invocation":{"address":"action.x.y","lifecycle_action_trigger":{"triggering_resource_address":"a.b","action_trigger_event":"AfterCreate"}}},{"reason":"","action_invocation":{"invoke_action_trigger":{}}}],"complete":false}`))
	f.Add([]byte(`{"format_version":"1.2","resource_changes":[{"address":"a.b","change":{"actions":["no-op"],"importing":{"id":"x"},"generated_config":"resource \"a\" \"b\" {\r\n\tk = \"\u001b[31mv\"\n}\n"}}]}`))
	f.Add([]byte("{\"format_version\":\"1.2\"}\nError: x\nWarning: y\n"))
	f.Fuzz(func(t *testing.T, input []byte) {
		formattest.CheckStable(t, func() (core.Result, error) { return Squeeze(input, DefaultOptions()) })
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
			for _, c := range r.Changes {
				if c.GeneratedConfig == "" {
					continue
				}
				formattest.CheckText(t, c.GeneratedConfig+"\n")
				if n := strings.Count(c.GeneratedConfig, "\n") + 1; n != c.GeneratedLines {
					t.Fatalf("%s: generated_config_lines = %d, config has %d lines", c.Address, c.GeneratedLines, n)
				}
			}
			for _, l := range p.Trailing {
				if !strings.Contains(res.Text, l.Text) {
					t.Fatalf("line after the JSON dropped: %q\n%s", l.Text, res.Text)
				}
				if l.Error && !slices.Contains(conditions(r), "error") {
					t.Fatalf("error line %q sets no error condition", l.Text)
				}
			}
			if _, err := core.Envelope("tfplan", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error after successful Build: %v", err)
			}
		}
		for _, list := range [][]PlanResource{p.ResourceChanges, p.ResourceDrift} {
			for _, rc := range list {
				attrs, _ := diffAttrs(rc.Change, true)
				for _, a := range attrs {
					if a.Before != nil && markedAt(rc.Change.BeforeSensitive, a.beforeAt) {
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

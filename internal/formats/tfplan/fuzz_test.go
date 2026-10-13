package tfplan

import (
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
)

func FuzzSqueeze(f *testing.F) {
	for _, data := range fixtures(f) {
		f.Add(data)
	}
	f.Add([]byte(`{"format_version":"1.2","resource_changes":[{"address":"a","change":{"actions":["update"],"before":{"a":[1,{"b":null}]},"after":{"a":[2]},"after_unknown":{"a":[false,true]},"before_sensitive":{"a":[true]},"replace_paths":[["a",0]]}}]}`))
	f.Add([]byte(`{"format_version":"1.2","resource_changes":[{"address":"a","change":{"actions":["update"],"before":1,"after":"x","after_unknown":[true],"before_sensitive":{"x":true}}}]}`))
	f.Fuzz(func(t *testing.T, input []byte) {
		for _, o := range []Options{
			DefaultOptions(),
			{MaxAttrs: 1, MaxResources: 1, IncludeNoop: true, NoValues: true, ListDrift: true},
		} {
			r, err := Squeeze(input, o)
			if err != nil {
				return
			}
			if _, err := core.Envelope("tfplan", len(input), r); err != nil {
				t.Fatalf("Envelope() error after successful Squeeze: %v", err)
			}
		}
	})
}

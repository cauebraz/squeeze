package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDecodeJSON(t *testing.T) {
	type doc struct {
		A int `json:"a"`
	}
	tests := map[string]struct {
		input   string
		want    doc
		rest    string
		wantErr bool
	}{
		"value only":         {input: `{"a":1}`, want: doc{A: 1}, rest: ""},
		"trailing newline":   {input: "{\"a\":2}\n", want: doc{A: 2}, rest: "\n"},
		"trailing text":      {input: "  {\"a\":3}\nError: plugin\n", want: doc{A: 3}, rest: "\nError: plugin\n"},
		"second value":       {input: `{"a":4}{"a":5}`, want: doc{A: 4}, rest: `{"a":5}`},
		"invalid":            {input: `{"a":`, wantErr: true},
		"empty":              {input: ``, wantErr: true},
		"wrong type":         {input: `{"a":"x"}`, wantErr: true},
		"text before value":  {input: `x{"a":1}`, wantErr: true},
		"rest keeps escapes": {input: "{}\x1b[31mwarn\x1b[0m", rest: "\x1b[31mwarn\x1b[0m"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var got doc
			rest, err := DecodeJSON([]byte(tc.input), &got)
			if (err != nil) != tc.wantErr {
				t.Fatalf("DecodeJSON() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				if rest != nil {
					t.Errorf("rest = %q on error, want nil", rest)
				}
				return
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("value (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tc.rest, string(rest)); diff != "" {
				t.Errorf("rest (-want +got):\n%s", diff)
			}
		})
	}
}

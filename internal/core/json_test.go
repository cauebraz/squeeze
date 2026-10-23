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

func TestFlaggedLines(t *testing.T) {
	tests := map[string]struct {
		input string
		want  []FlaggedLine
	}{
		"empty":          {input: "", want: nil},
		"blank lines":    {input: "\n  \n", want: nil},
		"plain text":     {input: "done\nwrote report.json\n", want: nil},
		"error prefix":   {input: "Error: Failed to load plugin\n", want: []FlaggedLine{{Text: "Failed to load plugin", Error: true}}},
		"warning prefix": {input: "WARNING: deprecated flag\r\n", want: []FlaggedLine{{Text: "deprecated flag"}}},
		"no prefix":      {input: "panic: runtime error\n", want: []FlaggedLine{{Text: "panic: runtime error", Error: true}}},
		"prefix only":    {input: "error:\n", want: []FlaggedLine{{Text: "error:", Error: true}}},
		"error wins":     {input: "warning: build failed", want: []FlaggedLine{{Text: "build failed", Error: true}}},
		"negated":        {input: "0 errors, no warnings\n", want: nil},
		"escapes":        {input: "\x1b[31mError:\x1b[0m bad\tthing\n", want: []FlaggedLine{{Text: "bad thing", Error: true}}},
		"several": {
			input: "ok\nError: a\nnote\nWarning: b\n",
			want:  []FlaggedLine{{Text: "a", Error: true}, {Text: "b"}},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.want, FlaggedLines([]byte(tc.input))); diff != "" {
				t.Errorf("FlaggedLines() (-want +got):\n%s", diff)
			}
		})
	}
}

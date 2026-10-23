package logscan

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

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

func TestLeadingLines(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input, lead, rest string
	}{
		"json first":            {`{"a":1}`, "", `{"a":1}`},
		"warning then json":     {"Warning: v1 thing is deprecated\n{\"a\":1}", "Warning: v1 thing is deprecated\n", `{"a":1}`},
		"blank and error lines": {"\nerror: boom\n\n  [1]", "\nerror: boom\n\n", "  [1]"},
		"plain line stops":      {"hello\n{\"a\":1}", "", "hello\n{\"a\":1}"},
		"no json":               {"Warning: only\n", "", "Warning: only\n"},
		"empty":                 {"", "", ""},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			lead, rest := LeadingLines([]byte(tt.input))
			if string(lead) != tt.lead || string(rest) != tt.rest {
				t.Errorf("LeadingLines(%q) = %q, %q; want %q, %q", tt.input, lead, rest, tt.lead, tt.rest)
			}
		})
	}
}

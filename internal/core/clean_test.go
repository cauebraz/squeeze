package core

import "testing"

func TestClean(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"plain":           "plain",
		"  padded\t":      "padded",
		"a\nb\r\nc":       "a b  c",
		"nul\x00byte":     "nulbyte",
		"esc\x1b[31mred":  "esc[31mred",
		"del\x7f":         "del",
		"c1\u0085control": "c1control",
		"café":            "café",
		"":                "",
	}
	for in, want := range tests {
		if got := Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}

package core

import "testing"

func TestClean(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"plain":                 "plain",
		"  padded\t":            "padded",
		"a\nb\r\nc":             "a b  c",
		"nul\x00byte":           "nulbyte",
		"esc\x1b[31mred":        "escred",
		"\x1b[1;31mbold\x1b[0m": "bold",
		"osc\x1b]8;;https://x\x07link\x1b]8;;\x1b\\": "osclink",
		"charset\x1b(Bset":                           "charsetset",
		"keypad\x1b=on":                              "keypadon",
		"open\x1b[":                                  "open",
		"open\x1b]title":                             "opentitle",
		"lone\x1b":                                   "lone",
		"del\x7f":                                    "del",
		"c1\u0085control":                            "c1control",
		"café":                                       "café",
		"":                                           "",
	}
	for in, want := range tests {
		if got := Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLine(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"  indented":               "  indented",
		"\tcode\t ":                "    code",
		"\x1b[32mPASS\x1b[0m ok  ": "PASS ok",
		"a\rb":                     "a b",
		"nul\x00":                  "nul",
		"":                         "",
	}
	for in, want := range tests {
		if got := Line(in); got != want {
			t.Errorf("Line(%q) = %q, want %q", in, got, want)
		}
	}
}

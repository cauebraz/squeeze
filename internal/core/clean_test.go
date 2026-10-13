package core

import (
	"strings"
	"testing"
	"time"
)

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
		"open\x1b]title":                             "open",
		"title\x1b]0;name\nnext":                     "title next",
		"dcs\x1bPq#0;2~-\x1b\\b":                     "dcsb",
		"apc\x1b_Ga=T;iVBO=\x1b\\b":                  "apcb",
		"pm\x1b^private\x1b\\b":                      "pmb",
		"sos\x1bXsos\x1b\\b":                         "sosb",
		"osc\x1b]0;t\u009cb":                         "oscb",
		"c1csi\u009b31mred":                          "c1csired",
		"c1osc\u009d0;title\u0007b":                  "c1oscb",
		"broken\x1b[31\x1b[0mb":                      "brokenb",
		"\ufeffbom":                                  "bom",
		"line\u2028sep\u2029par":                     "line sep par",
		"bidi\u202eevil\u2066x\u2069":                "bidievilx",
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

func TestCleanUnterminatedIsLinear(t *testing.T) {
	t.Parallel()
	input := strings.Repeat("\x1b]a", 200000)
	start := time.Now()
	if got := Clean(input); got != "" {
		t.Errorf("Clean() = %q, want empty", got[:min(len(got), 40)])
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("Clean() took %v on %d bytes", d, len(input))
	}
}

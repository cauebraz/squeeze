package logscan

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNegatedFlagWords(t *testing.T) {
	t.Parallel()
	tests := map[string][2]bool{
		"error: layer missing":                            {true, false},
		"1 warning, 0 errors":                             {false, true},
		"0 warnings suppressed":                           {false, false},
		"[001/400] ok (3 symbols, 0 warnings suppressed)": {false, false},
		"no errors found":                                 {false, false},
		"build finished without warnings":                 {false, false},
		"errors: 0, warnings: 2":                          {false, true},
		"warnings=0":                                      {false, false},
		"non-fatal":                                       {false, false},
		"Fatal error":                                     {true, false},
		"12 failed, 3 passed":                             {true, false},
		"zero failures":                                   {false, false},
		"ValueError: invalid literal":                     {true, false},
		"E   AssertionError":                              {true, false},
		"Caused by: java.lang.IllegalStateException: x":   {true, false},
		"Uncaught TypeError: x is not a function":         {true, false},
		"registered onError handler":                      {false, false},
		"calling handleError(ctx)":                        {false, false},
		"MirrorError retries 3":                           {false, false},
	}
	for line, want := range tests {
		if got := [2]bool{ErrorLine(line), WarningLine(line)}; got != want {
			t.Errorf("%q: error, warning = %v, want %v", line, got, want)
		}
	}
}

func TestStemFilterKeepsRegexpResults(t *testing.T) {
	t.Parallel()
	words := []string{
		"", " ", "0", ":", "=0", ": 0", "-", "no", "zero", "without", "non-", "ok", "terror", "erase", "fai", "warm",
		"error", "ERRORS", "ErrorS", "error\u017f",
		"fail", "FAILED", "fails", "failure", "failures", "fail\u017f", "fatal", "FATAL",
		"panic", "panicked", "PANICS", "panic\u017f", "exception", "Exceptions", "exception\u017f",
		"traceback", "TRACEBACK", "tracebac\u212a", "TRACEBAC\u212aS", "tracebac\u212a\u017f",
		"warn", "WARNED", "warnings", "warning\u017f", "warn\u017f",
		"ValueError:", "MyException", "IOError", "\u212a", "\u017f", "s", "k", "\u0130", "\u0131",
	}
	type verdict struct {
		Line           string
		Error, Warning bool
	}
	var got, want []verdict
	for _, a := range words {
		for _, b := range words {
			for _, line := range []string{a + b, a + " " + b, a + ": " + b} {
				got = append(got, verdict{line, ErrorLine(line), WarningLine(line)})
				want = append(want, verdict{line, flagged(line, errorWords) || errorName.MatchString(line), flagged(line, warningWords)})
			}
		}
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("stem filter changes results (-regexps +filter):\n%s", diff)
	}
}

func BenchmarkFlagWords(b *testing.B) {
	lines := []string{
		"    --- PASS: TestParse/nested_attributes (0.00s)",
		"2026-10-23T07:00:00Z INFO request served in 12ms path=/healthz status=200",
		"Step 4/12 : RUN go build -trimpath -o /out/app ./cmd/app",
		"module.network.aws_subnet.private[2]: Refreshing state... [id=subnet-0a1b2c3d]",
		"error: layer missing",
		"warning: deprecated base image",
	}
	b.ReportAllocs()
	for b.Loop() {
		for _, l := range lines {
			ErrorLine(l)
			WarningLine(l)
		}
	}
}

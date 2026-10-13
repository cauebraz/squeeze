package pytest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

const fixtureDir = "testdata"

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name, "input.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func lines(s ...string) []byte {
	return []byte(strings.Join(s, "\n") + "\n")
}

func report(t *testing.T, input []byte, o Options) Report {
	t.Helper()
	r, err := Squeeze(input, o)
	if err != nil {
		t.Fatal(err)
	}
	return r.Data.(Report)
}

func TestContract(t *testing.T) {
	t.Parallel()
	formattest.Contract(t, Format{}, fixtureDir)
}

func TestRealInputs(t *testing.T) {
	t.Parallel()
	formattest.Real(t, Format{}, "testdata/real")
}

func TestDetect(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input string
		want  bool
	}{
		"session":       {"===== test session starts =====\nplatform linux", true},
		"colored":       {"\x1b[1m===== test session starts =====\x1b[0m\n", true},
		"quiet":         {"..F  [100%]\n1 failed, 2 passed in 0.12s\n", true},
		"no tests":      {"no tests ran in 0.01s\n", true},
		"long run":      {"= 3 passed in 61.20s (0:01:01) =\n", true},
		"failures":      {"=== FAILURES ===\n___ test_a ___\n", true},
		"go test":       {"ok  \texample.com/p\t0.1s\n", false},
		"go json":       {`{"Action":"pass","Package":"p"}`, false},
		"other summary": {"3 passed in a row\n", false},
		"empty":         {"", false},
	}
	for name, tt := range tests {
		if got := (Format{}).Detect([]byte(tt.input)); got != tt.want {
			t.Errorf("%s: Detect() = %v, want %v", name, got, tt.want)
		}
	}
}

func TestParseRejectsOtherInput(t *testing.T) {
	t.Parallel()
	if _, err := Squeeze([]byte("collected nothing\n"), DefaultOptions()); err == nil {
		t.Error("Squeeze() returned no error")
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"default":     {"error", "fail", "skip", "warning", "xfail"},
		"interrupted": {"error", "fail", "interrupted"},
		"pass":        {},
		"empty":       {"no_tests"},
		"warn":        {"warning"},
		"deselected":  {"fail"},
		"notb":        {"error", "fail", "skip", "warning", "xfail"},
	}
	for name, want := range tests {
		r, err := Squeeze(fixture(t, name), DefaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, r.Conditions); diff != "" {
			t.Errorf("%s: conditions (-want +got):\n%s", name, diff)
		}
	}
}

func TestSummaryLine(t *testing.T) {
	t.Parallel()
	r := report(t, lines("1 failed, 2 subtests passed, 3 rerun, 1 warning in 0.10s"), DefaultOptions())
	want := Summary{Failed: 1, Warnings: 1, Other: map[string]int{"subtests passed": 2, "rerun": 3}, Duration: "0.10s", Line: "1 failed, 2 subtests passed, 3 rerun, 1 warning in 0.10s"}
	if diff := cmp.Diff(want, r.Summary); diff != "" {
		t.Errorf("summary (-want +got):\n%s", diff)
	}
}

func TestFailuresGetTheirNodeIDs(t *testing.T) {
	t.Parallel()
	input := lines(
		"=== FAILURES ===",
		"___ test_same ___",
		"E   assert 1 == 2",
		"___ test_same ___",
		"E   assert 3 == 4",
		"___ TestX.test_m[a-b] ___",
		"E   boom",
		"=== ERRORS ===",
		"___ ERROR at teardown of test_t ___",
		"E   RuntimeError",
		"=== short test summary info ===",
		"FAILED a/test_one.py::test_same - assert 1 == 2",
		"FAILED b/test_two.py::test_same - assert 3 == 4",
		"FAILED c.py::TestX::test_m[a-b] - boom",
		"ERROR c.py::test_t - RuntimeError",
		"FAILED d.py::test_only_in_summary - KeyError",
		"=== 4 failed, 1 error in 0.01s ===",
	)
	r := report(t, input, DefaultOptions())
	var got []string
	for _, f := range r.Failures {
		got = append(got, f.Kind+" "+f.Test+" "+f.When)
	}
	want := []string{
		"failed a/test_one.py::test_same ",
		"failed b/test_two.py::test_same ",
		"failed c.py::TestX::test_m[a-b] ",
		"error c.py::test_t teardown",
		"failed d.py::test_only_in_summary ",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failures (-want +got):\n%s", diff)
	}
	if last := r.Failures[4]; !last.Untracked || last.Message != "KeyError" {
		t.Errorf("summary-only failure = %+v", last)
	}
}

func TestTracebackSelection(t *testing.T) {
	t.Parallel()
	body := []string{
		"",
		"value = '12a'",
		"",
		"    def convert(value):",
		">       return int(value)",
		"               ^^^^^^^^^^",
		"E       ValueError: invalid literal",
		"E       ",
		"E         detail",
		"_ _ _ _ _ _ _ _",
		"more/test_more.py:13: ValueError",
		"x.py:3: in helper",
		"    helper_call()",
	}
	got, hidden := selectLines(body, false)
	want := []string{"> return int(value)", "E ValueError: invalid literal", "E   detail", "more/test_more.py:13: ValueError", "x.py:3: in helper", "helper_call()"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("lines (-want +got):\n%s", diff)
	}
	if hidden != 3 {
		t.Errorf("hidden = %d, want 3", hidden)
	}
	if got, hidden := selectLines([]string{"[XPASS(strict)] fixed"}, false); hidden != 0 || len(got) != 1 {
		t.Errorf("plain body = %v, %d", got, hidden)
	}
	if got, _ := selectLines(body, true); len(got) != 10 {
		t.Errorf("full body has %d lines, want 10", len(got))
	}
}

func TestWarnings(t *testing.T) {
	t.Parallel()
	input := lines(
		"=== warnings summary ===",
		"a.py::test_a",
		"a.py::test_a",
		"  a.py:3: DeprecationWarning: old",
		"    warn('old')",
		"",
		"b.py: 12 warnings",
		"  b.py:9: FutureWarning: soon",
		"    warn('soon')",
		"",
		"c.py::test_c",
		"  a.py:3: DeprecationWarning: old",
		"",
		"PytestConfigWarning: Unknown config option: foo",
		"",
		"-- Docs: https://docs.pytest.org/en/stable/how-to/capture-warnings.html",
		"=== 1 passed, 16 warnings in 0.01s ===",
	)
	r := report(t, input, DefaultOptions())
	want := []Warning{
		{Message: "a.py:3: DeprecationWarning: old", Count: 3},
		{Message: "b.py:9: FutureWarning: soon", Count: 12},
		{Message: "PytestConfigWarning: Unknown config option: foo"},
	}
	if diff := cmp.Diff(want, r.Warnings); diff != "" {
		t.Errorf("warnings (-want +got):\n%s", diff)
	}
	limited := report(t, input, Options{MaxWarnings: 1})
	if len(limited.Warnings) != 1 || limited.WarningsOmitted != 2 {
		t.Errorf("limited = %+v, omitted %d", limited.Warnings, limited.WarningsOmitted)
	}
}

func TestNotesKeepUnexpectedLines(t *testing.T) {
	t.Parallel()
	input := lines(
		"===== test session starts =====",
		"platform linux -- Python 3.13.1, pytest-9.1.1, pluggy-1.6.0",
		"rootdir: /home/dev/shop",
		"plugins: xdist-3.8.0",
		"created: 2/2 workers",
		"2 workers [3 items]",
		"",
		"tests/test_a.py ..                [ 66%]",
		"[gw0] [100%] PASSED tests/test_a.py::test_c",
		"INTERNALERROR> Traceback (most recent call last):",
		"INTERNALERROR>   File \"/home/dev/shop/plugin.py\", line 3",
		"===== 3 passed in 0.10s =====",
	)
	r := report(t, input, DefaultOptions())
	want := []string{"INTERNALERROR> Traceback (most recent call last):", "INTERNALERROR>   File \"plugin.py\", line 3"}
	if diff := cmp.Diff(want, r.Notes); diff != "" {
		t.Errorf("notes (-want +got):\n%s", diff)
	}
	if r.Version != "9.1.1" {
		t.Errorf("version = %q", r.Version)
	}
}

func TestInputCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := lines(
		"=== FAILURES ===",
		"___ test_a ___",
		"E   \rFAILED forged\x1b[31m",
		"> FAILED forged",
		"=== short test summary info ===",
		"FAILED t.py::test_a - x\x1b]0;title\x07",
		"=== 1 failed in 0.01s ===",
	)
	for _, o := range []Options{DefaultOptions(), {Full: true, All: true}} {
		r, err := Squeeze(input, o)
		if err != nil {
			t.Fatal(err)
		}
		formattest.CheckText(t, r.Text)
		if n := strings.Count(r.Text, "\nFAILED "); n != 1 {
			t.Errorf("%d FAILED lines, want 1:\n%s", n, r.Text)
		}
	}
}

func TestLimits(t *testing.T) {
	t.Parallel()
	r := report(t, fixture(t, "more"), Options{MaxTests: 3, MaxOutput: 2})
	if len(r.Failures) != 3 || r.FailuresOmitted != 7 {
		t.Errorf("failures = %d, omitted %d", len(r.Failures), r.FailuresOmitted)
	}
	for _, f := range r.Failures {
		if len(f.Lines) > 2 {
			t.Errorf("%s has %d lines", f.Test, len(f.Lines))
		}
	}
	if first := r.Failures[0]; first.Omitted == 0 || !strings.HasPrefix(first.Lines[0], "E ") {
		t.Errorf("the cut kept %v, want the error line", first.Lines)
	}
	if diff := cmp.Diff(r.Summary.Failed, len(r.Failures)+r.FailuresOmitted, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("failure count (-want +got):\n%s", diff)
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add(lines("=== FAILURES ===", "___ ___", "E", "=== warnings summary ===", "x: 3 warnings", "", "=== 1 failed in 0s ==="))
	f.Fuzz(func(t *testing.T, input []byte) {
		for _, o := range []Options{DefaultOptions(), {MaxTests: 1, MaxOutput: 1, MaxWarnings: 1, Full: true, All: true}} {
			res, err := Squeeze(input, o)
			if err != nil {
				return
			}
			formattest.CheckText(t, res.Text)
			r := res.Data.(Report)
			if o.MaxTests > 0 && len(r.Failures) > o.MaxTests {
				t.Fatalf("%d failures over the limit %d", len(r.Failures), o.MaxTests)
			}
			for _, fl := range r.Failures {
				if o.MaxOutput > 0 && len(fl.Lines) > o.MaxOutput {
					t.Fatalf("%d lines over the limit %d", len(fl.Lines), o.MaxOutput)
				}
			}
			if r.FailuresOmitted > 0 && !strings.Contains(res.Text, "(--max-tests 0)") {
				t.Fatalf("failures cut without a notice:\n%s", res.Text)
			}
			if again, _ := Squeeze(input, o); again.Text != res.Text {
				t.Fatal("output differs between runs")
			}
			if _, err := core.Envelope("pytest", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

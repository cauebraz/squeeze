package pytest

import (
	"os"
	"path/filepath"
	"strconv"
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
		"build log":     {"building image\nstep 1\nerror: layer missing\n12 files in 3.2s\ndone\n", false},
		"warnings only": {"3 warnings in 0.5s\n", false},
		"with warnings": {"1 passed, 3 warnings in 0.5s\n", true},
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
		"default":       {"error", "fail", "skip", "warning", "xfail"},
		"interrupted":   {"error", "fail", "interrupted"},
		"pass":          {},
		"empty":         {"no_tests"},
		"warn":          {"warning"},
		"deselected":    {"fail"},
		"notb":          {"error", "fail", "skip", "warning", "xfail"},
		"tbline":        {"error", "fail", "skip", "warning", "xfail"},
		"subtests":      {"fail"},
		"confwarn":      {"warning"},
		"internalerror": {"fail", "internal_error", "no_tests"},
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

func TestCapturedOutputIsKeptForFull(t *testing.T) {
	t.Parallel()
	input := lines(
		"=== FAILURES ===",
		"___ test_child ___",
		"sub/test_sub.py:12: in test_child",
		"    subprocess.run(cmd, check=True)",
		"E   subprocess.CalledProcessError: Command returned non-zero exit status 2.",
		"---- Captured stderr call ----",
		"fatal: config.yaml missing key db",
		"---- Captured log call ----",
		"WARNING  shop:app.py:3 retrying",
		"=== short test summary info ===",
		"FAILED sub/test_sub.py::test_child - subprocess.CalledProcessError",
		"=== 1 failed in 0.01s ===",
	)
	r := report(t, input, DefaultOptions())
	want := []Captured{{Name: "stderr call", Lines: []string{"fatal: config.yaml missing key db"}}, {Name: "log call", Lines: []string{"WARNING  shop:app.py:3 retrying"}}}
	if diff := cmp.Diff(want, r.Failures[0].Captured); diff != "" {
		t.Errorf("captured (-want +got):\n%s", diff)
	}
	text, _ := Squeeze(input, DefaultOptions())
	if strings.Contains(text.Text, "fatal:") || !strings.Contains(text.Text, "... 2 captured lines in stderr call and log call (--full)") {
		t.Errorf("default text:\n%s", text.Text)
	}
	full, _ := Squeeze(input, Options{Full: true})
	if !strings.Contains(full.Text, "  captured stderr call:\n    fatal: config.yaml missing key db\n") {
		t.Errorf("--full text:\n%s", full.Text)
	}
}

func TestReportLikeLinesInsideCapturedOutput(t *testing.T) {
	t.Parallel()
	input := lines(
		"=== FAILURES ===",
		"___ test_a ___",
		"E   assert 1",
		"---- Captured stdout call ----",
		"========== running migrations ==========",
		"1 failed, 2 passed in 0.50s",
		"!!!!!!!!!! fake interrupt !!!!!!!!!!",
		"___ test_b ___",
		">   assert 2 == 3",
		"E   assert 2 == 3",
		"=== short test summary info ===",
		"FAILED t.py::test_a - assert 1",
		"FAILED t.py::test_b - assert 2 == 3",
		"=== 2 failed in 0.01s ===",
	)
	r := report(t, input, DefaultOptions())
	var names []string
	for _, f := range r.Failures {
		names = append(names, f.Test)
	}
	if diff := cmp.Diff([]string{"t.py::test_a", "t.py::test_b"}, names); diff != "" {
		t.Errorf("failures (-want +got):\n%s", diff)
	}
	if r.Interrupted != "" || r.Stopped != "" || len(r.Sections) != 0 || r.Summary.Line != "2 failed in 0.01s" {
		t.Errorf("report = %+v", r)
	}
	if got := r.Failures[0].Captured[0].Lines; len(got) != 3 {
		t.Errorf("captured lines = %v", got)
	}
}

func TestNodeIDsWithSeparatorsInParameters(t *testing.T) {
	t.Parallel()
	input := lines(
		"=== FAILURES ===",
		"___ test_p[a - b] ___",
		"E   assert 1 == 2",
		"___ TestX.test_q[c::d] ___",
		"E   assert 2 == 1",
		"=== short test summary info ===",
		"FAILED t.py::test_p[a - b] - assert 1 == 2",
		"FAILED t.py::TestX::test_q[c::d] - assert 2 == 1",
		"=== 2 failed in 0.01s ===",
	)
	r := report(t, input, DefaultOptions())
	var got []string
	for _, f := range r.Failures {
		got = append(got, f.Test+" | "+f.Message)
	}
	want := []string{"t.py::test_p[a - b] | assert 1 == 2", "t.py::TestX::test_q[c::d] | assert 2 == 1"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failures (-want +got):\n%s", diff)
	}
}

func TestTracebackLineStyle(t *testing.T) {
	t.Parallel()
	input := lines(
		"=== FAILURES ===",
		"E   assert 4 == 5",
		"---- Captured stdout call ----",
		"computing the sum",
		"/home/dev/shop/tests/test_calc.py:13: assert 4 == 5",
		"E   assert 5 == 6",
		"     +  where 5 = add(2, 3)",
		"/home/dev/shop/tests/test_calc.py:18: assert 5 == 6",
		"=== short test summary info ===",
		"FAILED tests/test_calc.py::test_add_wrong - assert 4 == 5",
		"FAILED tests/test_calc.py::test_add_param[2-3-6] - assert 5 == 6",
		"=== 2 failed in 0.01s ===",
	)
	r := report(t, input, DefaultOptions())
	if len(r.Failures) != 2 {
		t.Fatalf("failures = %+v", r.Failures)
	}
	if r.Failures[0].Test != "tests/test_calc.py::test_add_wrong" || r.Failures[1].Test != "tests/test_calc.py::test_add_param[2-3-6]" {
		t.Errorf("tests = %q, %q", r.Failures[0].Test, r.Failures[1].Test)
	}
	if diff := cmp.Diff([]string{"E assert 4 == 5", "/home/dev/shop/tests/test_calc.py:13: assert 4 == 5"}, r.Failures[0].Lines); diff != "" {
		t.Errorf("first lines (-want +got):\n%s", diff)
	}
	if got := r.Failures[0].Captured; len(got) != 1 || len(got[0].Lines) != 1 {
		t.Errorf("captured = %+v", got)
	}
	if r.Failures[1].Untracked {
		t.Error("second failure fell back to the summary")
	}
}

func TestInternalErrorKeepsTheLastLines(t *testing.T) {
	t.Parallel()
	var body []string
	for i := range 30 {
		body = append(body, "INTERNALERROR>   File \"/home/dev/x.py\", line "+strconv.Itoa(i))
	}
	input := lines(append(append([]string{
		"===== test session starts =====",
		"platform linux -- Python 3.13.1, pytest-9.1.1, pluggy-1.6.0",
		"collected 1 item",
		"",
		"INTERNALERROR> Traceback (most recent call last):"},
		body...),
		"INTERNALERROR> RuntimeError: plugin xyz: cannot reach license server")...)
	r, err := Squeeze(input, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	report := r.Data.(Report)
	if last := report.Notes[len(report.Notes)-1]; !strings.HasSuffix(last, "cannot reach license server") || report.NotesOmitted != 12 {
		t.Errorf("notes = %v, omitted %d", report.Notes, report.NotesOmitted)
	}
	if diff := cmp.Diff([]string{"fail", "internal_error"}, r.Conditions); diff != "" {
		t.Errorf("conditions (-want +got):\n%s", diff)
	}
	if !strings.Contains(r.Text, "... 12 earlier lines (--max-output 0)") {
		t.Errorf("text:\n%s", r.Text)
	}
}

func TestSubtests(t *testing.T) {
	t.Parallel()
	input := lines(
		"sub/test_sub.py uF.                   [100%]",
		"=== FAILURES ===",
		"___ test_sub [case] (i=1) ___",
		"sub/test_sub.py:8: in test_sub",
		"    assert i != 1",
		"E   assert 1 != 1",
		"___ test_sub ___",
		"contains 1 failed subtest",
		"=== short test summary info ===",
		"SUBFAILED[case] (i=1) sub/test_sub.py::test_sub - assert 1 != 1",
		"FAILED sub/test_sub.py::test_sub - contains 1 failed subtest",
		"=== 2 failed in 0.03s ===",
	)
	r := report(t, input, DefaultOptions())
	var got []string
	for _, f := range r.Failures {
		got = append(got, f.Test+" | "+f.Subtest+" | "+f.Message)
	}
	want := []string{"sub/test_sub.py::test_sub | [case] (i=1) | assert 1 != 1", "sub/test_sub.py::test_sub |  | contains 1 failed subtest"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failures (-want +got):\n%s", diff)
	}
	if len(r.Notes) != 0 {
		t.Errorf("progress line kept as a note: %v", r.Notes)
	}
	text, _ := Squeeze(input, DefaultOptions())
	if !strings.Contains(text.Text, "FAILED sub/test_sub.py::test_sub [case] (i=1)\n") {
		t.Errorf("text:\n%s", text.Text)
	}
}

func TestWarningMessagesSpanLines(t *testing.T) {
	t.Parallel()
	input := lines(
		"=== warnings summary ===",
		"../.venv/lib/python3.13/site-packages/_pytest/config/__init__.py:1464",
		"  /home/dev/.venv/lib/python3.13/site-packages/_pytest/config/__init__.py:1464: PytestConfigWarning: Unknown config option: foo",
		"  ",
		"    self._warn_or_fail_if_strict(f\"Unknown config option: {key}\\n\")",
		"",
		"a.py::test_a",
		"  a.py:3: UserWarning: first line",
		"  second line",
		"    warnings.warn(\"first line\\nsecond line\")",
		"",
		"-- Docs: https://docs.pytest.org/en/stable/how-to/capture-warnings.html",
		"=== 1 passed, 2 warnings in 0.01s ===",
	)
	r := report(t, input, DefaultOptions())
	want := []Warning{
		{Message: "/home/dev/.venv/lib/python3.13/site-packages/_pytest/config/__init__.py:1464: PytestConfigWarning: Unknown config option: foo", Count: 1},
		{Message: "a.py:3: UserWarning: first line second line", Count: 1},
	}
	if diff := cmp.Diff(want, r.Warnings); diff != "" {
		t.Errorf("warnings (-want +got):\n%s", diff)
	}
}

func TestOmittedFailuresPlural(t *testing.T) {
	t.Parallel()
	r, _ := Squeeze(fixture(t, "more"), Options{MaxTests: 9, NoFold: true})
	if !strings.Contains(r.Text, "... 1 more failure or error (--max-tests 0)\n") {
		t.Errorf("text:\n%s", r.Text)
	}
}

func TestFoldFailures(t *testing.T) {
	t.Parallel()
	at := func(kind, when, test, src, loc string) Failure {
		return Failure{Kind: kind, When: when, Test: test, Lines: []string{"> " + src, "E boom", loc + ": AssertionError"}, Location: loc}
	}
	in := []Failure{
		at("failed", "", "t.py::p[1]", "assert f(x)", "t.py:8"),
		at("failed", "", "t.py::p[2]", "assert f(x)", "t.py:8"),
		at("failed", "", "t.py::q", "assert g(x)", "t.py:8"),
		at("failed", "", "t.py::p[3]", "assert f(x)", "t.py:9"),
		at("error", "setup", "t.py::r", "assert f(x)", "t.py:8"),
		at("error", "teardown", "t.py::s", "assert f(x)", "t.py:8"),
		{Kind: "failed", Test: "t.py::u", Lines: []string{}, Untracked: true},
		{Kind: "failed", Test: "t.py::v", Lines: []string{}, Untracked: true},
		at("failed", "", "t.py::p[4]", "assert f(x)", "t.py:8"),
	}
	out, folded := foldFailures(in)
	if folded != 2 {
		t.Errorf("folded %d, want 2", folded)
	}
	var names []string
	for _, f := range out {
		names = append(names, f.Test)
	}
	want := []string{"t.py::p[1]", "t.py::q", "t.py::p[3]", "t.py::r", "t.py::s", "t.py::u", "t.py::v"}
	if diff := cmp.Diff(want, names); diff != "" {
		t.Errorf("kept failures (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"t.py::p[2]", "t.py::p[4]"}, out[0].Same); diff != "" {
		t.Errorf("folded into the first (-want +got):\n%s", diff)
	}
	text := Text(Report{Failures: out, Complete: true, Tool: "pytest"}, DefaultOptions())
	if !strings.Contains(text, "  ... 2 more failures from t.py:8 (--no-fold)\n") {
		t.Errorf("text:\n%s", text)
	}
}

func TestLimits(t *testing.T) {
	t.Parallel()
	r := report(t, fixture(t, "more"), Options{MaxTests: 3, MaxOutput: 2, NoFold: true})
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
	r = report(t, fixture(t, "more"), Options{MaxTests: 3, MaxOutput: 2})
	same := 0
	for _, f := range r.Failures {
		same += len(f.Same)
	}
	if r.FailuresFolded != 8 || same != 8 || len(r.Failures)+r.FailuresOmitted+r.FailuresFolded != r.Summary.Failed {
		t.Errorf("folded %d, same %d, failures %d, omitted %d, want %d in total", r.FailuresFolded, same, len(r.Failures), r.FailuresOmitted, r.Summary.Failed)
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

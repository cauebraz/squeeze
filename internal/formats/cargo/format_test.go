package cargo

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
	"github.com/google/go-cmp/cmp"
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

func report(t *testing.T, input string, o Options) Report {
	t.Helper()
	r, err := Squeeze([]byte(input), o)
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
		"compiler message":     {`{"reason":"compiler-message","package_id":"p","message":{}}`, true},
		"after progress lines": {"   Compiling shop v0.1.0 (/w)\n" + `{"reason":"build-finished","success":true}`, true},
		"artifact":             {`{"reason":"compiler-artifact","package_id":"p"}`, true},
		"build script":         {`{"reason":"build-script-executed","package_id":"p"}`, true},
		"other reason":         {`{"reason":"timing-info","package_id":"p"}`, false},
		"reason not first":     {`{"package_id":"p","reason":"compiler-message"}`, false},
		"libtest":              {"\nrunning 2 tests\ntest a ... ok\ntest b ... ok\n", true},
		"libtest quiet":        {"\nrunning 2 tests\n..\ntest result: ok. 2 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s\n", true},
		"running alone":        {"running 2 tests\n", false},
		"test line alone":      {"test a ... ok\n", false},
		"could not compile":    {"error: could not compile `shop` (lib) due to 1 previous error\n", true},
		"finished":             {"    Finished `dev` profile [unoptimized + debuginfo] target(s) in 0.20s\n", true},
		"old finished":         {"    Finished dev [unoptimized + debuginfo] target(s) in 0.20s\n", true},
		"rustc error only":     {"error[E0308]: mismatched types\n --> src/lib.rs:1:1\n", false},
		"go test text":         {"--- FAIL: TestA (0.00s)\nFAIL\nok  \texample.com/p\t0.1s\n", false},
		"go test json":         {`{"Time":"2026-01-01T00:00:00Z","Action":"run","Package":"p","Test":"TestA"}`, false},
		"pytest":               {"============================= test session starts ==============================\ncollected 2 items\n", false},
		"eslint":               {`[{"filePath":"/a.js","messages":[],"errorCount":0}]`, false},
		"gh run view":          {"build\tRun cargo test\t2026-10-13T22:20:23.7522810Z running 2 tests\nbuild\tRun cargo test\t2026-10-13T22:20:23.7522810Z test a ... ok\n", false},
		"plain text":           {"hello\nworld\n", false},
	}
	for name, tt := range tests {
		if got := (Format{}).Detect([]byte(tt.input)); got != tt.want {
			t.Errorf("%s: Detect() = %v, want %v", name, got, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"empty":        "",
		"plain text":   "hello\nworld\n",
		"progress":     "   Compiling shop v0.1.0 (/w)\n",
		"broken json":  `{"reason":"compiler-message",`,
		"other reason": `{"reason":"timing-info"}`,
	} {
		if _, err := Squeeze([]byte(input), DefaultOptions()); !errors.Is(err, ErrNotCargo) {
			t.Errorf("%s: Squeeze() error = %v, want ErrNotCargo", name, err)
		}
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"build-errors":       {"build_failed", "error", "fail", "fixable", "warning"},
		"build-script":       {"build_failed", "error", "fail"},
		"build-warnings":     {"warning"},
		"check-clean":        {},
		"clippy":             {"build_failed", "error", "fail", "fixable", "warning"},
		"test":               {"fail", "ignored", "warning"},
		"test-json":          {"fail", "fixable", "ignored", "warning"},
		"test-stdout":        {"fail", "ignored"},
		"test-compile-error": {"build_failed", "error", "fail", "warning"},
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

func TestHumanCodeLineIsTheErrorLine(t *testing.T) {
	t.Parallel()
	input := "error[E0308]: mismatched types\n" +
		"  --> src/account.rs:35:9\n" +
		"   |\n" +
		"34 |     pub fn entry_count(&self) -> u32 {\n" +
		"   |                                  --- expected `u32` because of return type\n" +
		"35 |         self.entries.len()\n" +
		"   |         ^^^^^^^^^^^^^^^^^^ expected `u32`, found `usize`\n" +
		"\n" +
		"error: could not compile `ledger` (lib) due to 1 previous error\n"
	r := report(t, input, DefaultOptions())
	if len(r.Diagnostics) == 0 || r.Diagnostics[0].Code != "self.entries.len()" {
		t.Errorf("diagnostics = %+v", r.Diagnostics)
	}
}

func TestFixableFollowsMachineApplicable(t *testing.T) {
	t.Parallel()
	r := report(t, string(fixture(t, "clippy")), DefaultOptions())
	fixable := map[string]bool{}
	for _, d := range r.Diagnostics {
		fixable[d.ID] = d.Fixable
	}
	want := map[string]bool{"clippy::eq_op": false, "clippy::needless_range_loop": false, "clippy::needless_return": true, "clippy::ptr_arg": false, "dead_code": false}
	if diff := cmp.Diff(want, fixable); diff != "" {
		t.Errorf("fixable by code (-want +got):\n%s", diff)
	}
}

func TestTestCounts(t *testing.T) {
	t.Parallel()
	r := report(t, string(fixture(t, "test")), DefaultOptions())
	if diff := cmp.Diff(Counts{Passed: 5, Failed: 5, Ignored: 1}, r.Tests); diff != "" {
		t.Errorf("tests (-want +got):\n%s", diff)
	}
	var names []string
	for _, tg := range r.Targets {
		names = append(names, tg.Name+" "+tg.Result+" "+tg.Rerun)
	}
	want := []string{"unittests src/lib.rs (gen) ok ", "unittests src/lib.rs (shop) FAILED -p shop --lib", "tests/checkout.rs FAILED -p shop --test checkout", "doc-tests gen ok ", "doc-tests shop FAILED -p shop --doc"}
	if diff := cmp.Diff(want, names); diff != "" {
		t.Errorf("targets (-want +got):\n%s", diff)
	}
	if len(r.Failures) != r.Tests.Failed {
		t.Errorf("%d failures listed, %d counted", len(r.Failures), r.Tests.Failed)
	}
	f := r.Failures[1]
	wantF := Failure{Target: "unittests src/lib.rs (shop)", Test: "tests::labels_match", File: "shop/src/lib.rs", Line: 51, Column: 9,
		Message: []string{"assertion `left == right` failed: labels differ for cart 7", "  left: [\"pen\", \"ink\"]", " right: [\"pen\", \"paper\"]"}, Output: []string{}, index: 1}
	if diff := cmp.Diff(wantF, f, cmp.AllowUnexported(Failure{})); diff != "" {
		t.Errorf("failure (-want +got):\n%s", diff)
	}
}

func TestCrashedTestBinary(t *testing.T) {
	t.Parallel()
	input := "     Running unittests src/lib.rs (target/debug/deps/shop-17f604ca6170e442)\n\n" +
		"running 3 tests\ntest a ... ok\ntest b - should panic ... FAILED\n" +
		"error: test failed, to rerun pass `-p shop --lib`\n\nCaused by:\n  process didn't exit successfully: `x` (signal: 6, SIGABRT: process abort signal)\n" +
		"error: 1 target failed:\n    `-p shop --lib`\n"
	r := report(t, input, DefaultOptions())
	want := Target{Name: "unittests src/lib.rs (shop)", Passed: 1, Failed: 1, Rerun: "-p shop --lib", Cause: []string{"process didn't exit successfully: `x` (signal: 6, SIGABRT: process abort signal)"}}
	if diff := cmp.Diff([]Target{want}, r.Targets); diff != "" {
		t.Errorf("targets (-want +got):\n%s", diff)
	}
	if r.Status != "fail" || len(r.Failures) != 1 || r.Failures[0].Test != "b" || r.Errors != 0 {
		t.Errorf("report = %+v", r)
	}
	text, _ := Squeeze([]byte(input), DefaultOptions())
	if !strings.Contains(text.Text, "failed target unittests src/lib.rs (shop): did not finish after 1 passed, 1 failed; rerun: cargo test -p shop --lib\n  process didn't exit") {
		t.Errorf("text:\n%s", text.Text)
	}
}

func TestOldPanicFormat(t *testing.T) {
	t.Parallel()
	input := "running 1 test\ntest t ... FAILED\n\nfailures:\n\n---- t stdout ----\nthread 't' panicked at 'boom', src/lib.rs:3:5\nnote: run with `RUST_BACKTRACE=1` environment variable to display a backtrace\n\n\nfailures:\n    t\n\n" +
		"test result: FAILED. 0 passed; 1 failed; 0 ignored; 0 measured; 2 filtered out; finished in 0.00s\n"
	r := report(t, input, DefaultOptions())
	want := []Failure{{Target: "test binary 1", Test: "t", File: "src/lib.rs", Line: 3, Column: 5, Message: []string{"boom"}, Output: []string{}}}
	if diff := cmp.Diff(want, r.Failures, cmp.AllowUnexported(Failure{})); diff != "" {
		t.Errorf("failures (-want +got):\n%s", diff)
	}
	if r.Tests.FilteredOut != 2 || r.Build != "ok" {
		t.Errorf("report = %+v", r)
	}
}

func TestBacktraceGoesToOutput(t *testing.T) {
	t.Parallel()
	input := "running 1 test\ntest t ... FAILED\n\nfailures:\n\n---- t stdout ----\nbefore\nthread 't' (12) panicked at src/lib.rs:3:5:\nboom\nstack backtrace:\n   0: frame\n\nfailures:\n    t\n\n" +
		"test result: FAILED. 0 passed; 1 failed; 0 ignored; 1 measured; 0 filtered out; finished in 0.00s\n"
	r := report(t, input, Options{MaxOutput: 3})
	f := r.Failures[0]
	if diff := cmp.Diff([]string{"boom"}, f.Message); diff != "" {
		t.Errorf("message (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"before", "stack backtrace:", "   0: frame"}, f.Output); diff != "" {
		t.Errorf("output (-want +got):\n%s", diff)
	}
	text, _ := Squeeze([]byte(input), Options{MaxOutput: 3})
	if !strings.Contains(text.Text, "1 measured") || !strings.Contains(text.Text, "    ... 1 more line (--max-output 0)\n") {
		t.Errorf("text:\n%s", text.Text)
	}
}

func TestCompilerMessageLevels(t *testing.T) {
	t.Parallel()
	msg := func(level string) string {
		return `{"reason":"compiler-message","message":{"message":"m ` + level + `","code":null,"level":"` + level + `","spans":[],"children":[]}}` + "\n"
	}
	input := msg("error: internal compiler error") + msg("note") + msg("help") + msg("failure-note") + msg("future-level") +
		`{"reason":"compiler-message","message":null}` + "\n" + `{"reason":"build-finished","success":null}` + "\n"
	r := report(t, input, DefaultOptions())
	var got []string
	for _, d := range r.Diagnostics {
		got = append(got, d.Severity+" "+d.Summary)
	}
	if diff := cmp.Diff([]string{"error m error: internal compiler error", "warning m future-level"}, got); diff != "" {
		t.Errorf("diagnostics (-want +got):\n%s", diff)
	}
	if r.Build != "" || r.Status != "fail" {
		t.Errorf("build %q status %q", r.Build, r.Status)
	}
}

func TestSpansAndSuggestions(t *testing.T) {
	t.Parallel()
	input := `{"reason":"compiler-message","message":{"message":"m","code":{"code":"E0277"},"level":"error","spans":[` +
		`{"file_name":"src/a.rs","line_start":4,"column_start":2,"is_primary":false,"label":"required here","text":[]},` +
		`{"file_name":"src/b.rs","line_start":9,"column_start":3,"is_primary":true,"label":null,"text":[{"text":"    let x = y;"}]}],` +
		`"children":[{"message":"try","level":"help","spans":[{"file_name":"src/b.rs","line_start":9,"column_start":3,"is_primary":true,"suggested_replacement":"a\nb","suggestion_applicability":"MaybeIncorrect"}],"children":[]}]}}`
	r := report(t, input, DefaultOptions())
	d := r.Diagnostics[0]
	if d.File != "src/b.rs" || d.Line != 9 || d.Column != 3 || d.Code != "let x = y;" || d.Detail != "required here (src/a.rs:4); help: try" || d.Fixable {
		t.Errorf("diagnostic = %+v", d)
	}
	if got := primary([]Span{{FileName: "x"}}); got == nil || got.FileName != "x" {
		t.Errorf("primary() without a primary span = %+v", got)
	}
}

func TestDuplicatesFold(t *testing.T) {
	t.Parallel()
	one := `{"reason":"compiler-message","message":{"message":"unused","code":{"code":"dead_code"},"level":"warning","spans":[{"file_name":"src/a.rs","line_start":1,"column_start":1,"is_primary":true}],"children":[]}}` + "\n"
	r := report(t, one+one+`{"reason":"build-finished","success":true}`, DefaultOptions())
	if len(r.Diagnostics) != 1 || r.Diagnostics[0].Count != 2 || r.Warnings != 2 || r.Status != "ok" {
		t.Errorf("report = %+v", r)
	}
}

func TestDiagnosticLimitNamesCodes(t *testing.T) {
	t.Parallel()
	r := report(t, string(fixture(t, "build-errors")), Options{MaxDiagnostics: 1})
	if r.DiagnosticsOmitted != 3 || len(r.Diagnostics) != 1 {
		t.Fatalf("omitted %d, kept %d", r.DiagnosticsOmitted, len(r.Diagnostics))
	}
	want := []core.Count{{Name: "E0382", Count: 1}, {Name: "unused_imports", Count: 1}, {Name: "unused_variables", Count: 1}}
	if diff := cmp.Diff(want, r.OmittedByCode); diff != "" {
		t.Errorf("omitted by code (-want +got):\n%s", diff)
	}
	r = report(t, "error: something broke\nerror: other\n", Options{MaxDiagnostics: 1})
	if diff := cmp.Diff([]core.Count{{Name: noCode, Count: 1}}, r.OmittedByCode); diff != "" {
		t.Errorf("omitted without a code (-want +got):\n%s", diff)
	}
}

func TestHumanDiagnostics(t *testing.T) {
	t.Parallel()
	input := "warning: unneeded `return` statement\n  --> src/lib.rs:10:5\n   |\n10 |     return sum;\n   |     ^^^^^^^^^^\n   |\n" +
		"   = help: for further information visit https://rust-lang.github.io/rust-clippy\n   = note: `#[warn(clippy::needless_return)]` on by default\n" +
		"help: remove `return`\n   |\n10 -     return sum;\n\n" +
		"error: failed to parse manifest at `/w/Cargo.toml`\n\nCaused by:\n  missing field `name`\n" +
		"error: no code here\n"
	r := report(t, input, DefaultOptions())
	var got []string
	for _, d := range r.Diagnostics {
		got = append(got, d.Severity+"|"+d.ID+"|"+d.Detail+"|"+d.Code)
	}
	want := []string{"error||missing field `name`|", "error|||", "warning|clippy::needless_return|help: remove `return`|return sum;"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("diagnostics (-want +got):\n%s", diff)
	}
}

func TestUnitName(t *testing.T) {
	t.Parallel()
	tests := map[[2]string]string{
		{"unittests src/lib.rs", "/w/target/debug/deps/shop-17f604ca6170e442"}:       "unittests src/lib.rs (shop)",
		{"unittests src/main.rs", `C:\w\target\debug\deps\cli-17f604ca6170e442.exe`}: "unittests src/main.rs (cli)",
		{"tests/api.rs", "/w/target/debug/deps/api-17f604ca6170e442"}:                "tests/api.rs",
	}
	for in, want := range tests {
		if got := unitName(in[0], in[1]); got != want {
			t.Errorf("unitName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := `{"reason":"compiler-message","message":{"message":"m\nerror: forged","code":{"code":"E1\nerror: forged"},"level":"error","spans":[{"file_name":"a.rs\nerror: forged","line_start":1,"column_start":1,"is_primary":true,"label":"l\u001b[31m\nerror: forged","text":[{"text":"x\nerror: forged"}]}],"children":[]}}` + "\n" +
		"running 1 test\ntest t ... FAILED\n\nfailures:\n\n---- t\x1b[31m stdout ----\nthread 't' panicked at a.rs:1:1:\nmsg\x1b]0;title\x07\n\n" +
		"test result: FAILED. 0 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s\n"
	r, err := Squeeze([]byte(input), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	formattest.CheckText(t, r.Text)
	for _, line := range strings.Split(r.Text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "error: forged") {
			t.Errorf("input forged a line: %q\n%s", line, r.Text)
		}
	}
	if !strings.Contains(r.Text, "  fail t (a.rs:1:1)\n    msg\n") {
		t.Errorf("text:\n%s", r.Text)
	}
}

func TestDataPayload(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "check-clean"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	out, err := core.Envelope("cargo", 10, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "null") {
		t.Errorf("envelope has a null collection: %s", out)
	}
	r, _ = Squeeze(fixture(t, "build-errors"), DefaultOptions())
	if strings.Contains(r.Text, "Erroneous code example") || strings.Contains(r.Text, " --> ") {
		t.Errorf("rendered text or explanation reached the output:\n%s", r.Text)
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte("running 1 test\n---- x stdout ----\nfailures:\n    y\nerror: test failed, to rerun pass `-p a`\n\nCaused by:\n  z\n"))
	f.Fuzz(func(t *testing.T, input []byte) {
		run, err := Parse(input)
		if err != nil {
			return
		}
		for _, o := range []Options{DefaultOptions(), {MaxDiagnostics: 1, MaxFailures: 1, MaxOutput: 1}} {
			r := Build(run, o)
			res := core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			formattest.CheckNotices(t, res.Text)
			shown := 0
			for _, d := range r.Diagnostics {
				shown += max(d.Count, 1)
			}
			if shown+r.DiagnosticsOmitted != len(run.Diagnostics) || r.Errors+r.Warnings != len(run.Diagnostics) {
				t.Fatalf("%d diagnostics shown, %d omitted, %d+%d counted, %d in the input", shown, r.DiagnosticsOmitted, r.Errors, r.Warnings, len(run.Diagnostics))
			}
			if len(r.Failures)+r.FailuresOmitted != len(run.Failures) {
				t.Fatalf("%d failures listed, %d omitted, %d parsed", len(r.Failures), r.FailuresOmitted, len(run.Failures))
			}
			if o.MaxFailures > 0 && len(r.Failures) > o.MaxFailures {
				t.Fatalf("%d failures over the limit %d", len(r.Failures), o.MaxFailures)
			}
			if r.DiagnosticsOmitted > 0 && !strings.Contains(res.Text, "(--max-diagnostics 0)") {
				t.Fatalf("diagnostics cut without a notice:\n%s", res.Text)
			}
			if r.FailuresOmitted > 0 && !strings.Contains(res.Text, "(--max-failures 0)") {
				t.Fatalf("failures cut without a notice:\n%s", res.Text)
			}
			if (r.Errors > 0 || r.Tests.Failed > 0 || r.Build == "failed") && r.Status != "fail" {
				t.Fatalf("status %q with %d errors, %d failed tests, build %q", r.Status, r.Errors, r.Tests.Failed, r.Build)
			}
			if _, err := core.Envelope("cargo", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

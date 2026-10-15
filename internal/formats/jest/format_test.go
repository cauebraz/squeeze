package jest

import (
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
	data, err := os.ReadFile(filepath.Join(fixtureDir, name, "input.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func squeeze(t *testing.T, input []byte, o Options) (core.Result, Report) {
	t.Helper()
	r, err := Squeeze(input, o)
	if err != nil {
		t.Fatal(err)
	}
	return r, r.Data.(Report)
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
		"jest":              {`{"numFailedTestSuites":0,"numFailedTests":0,"numTotalTests":0,"testResults":[]}`, true},
		"vitest":            {`{"numTotalTestSuites":0,"numFailedTestSuites":0,"numTotalTests":0,"testResults":[]}`, true},
		"pretty":            {"{\n  \"numFailedTestSuites\": 0,\n  \"numTotalTests\": 0,\n  \"testResults\": []\n}", true},
		"no test results":   {`{"numFailedTestSuites":0,"numTotalTests":0}`, false},
		"other first key":   {`{"testResults":[],"numFailedTestSuites":0,"numTotalTests":0}`, false},
		"eslint":            {`[{"filePath":"/a.js","messages":[],"errorCount":0}]`, false},
		"sarif":             {`{"version":"2.1.0","runs":[]}`, false},
		"terraform outputs": {`{"num":{"sensitive":false,"type":"number","value":1}}`, false},
		"jest text":         {"FAIL test/a.test.js\n  numTotalTests testResults numFailedTestSuites", false},
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
		"truncated":       `{"numTotalTests":`,
		"no test results": `{"numTotalTests":0}`,
		"wrong type":      `{"numTotalTests":0,"testResults":{}}`,
		"file no name":    `{"numTotalTests":0,"testResults":[{"assertionResults":[]}]}`,
		"empty":           ``,
	} {
		if _, err := Squeeze([]byte(input), DefaultOptions()); err == nil {
			t.Errorf("%s: Squeeze() returned no error", name)
		}
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"run":          {"error", "fail", "skip", "snapshot", "todo"},
		"vitest":       {"error", "fail", "skip", "snapshot", "todo"},
		"open-handles": {"warning"},
		"run-error":    {"error", "fail", "snapshot"},
		"vitest-pass":  {},
		"many":         {"fail"},
	}
	for name, want := range tests {
		r, _ := squeeze(t, fixture(t, name), DefaultOptions())
		if diff := cmp.Diff(want, r.Conditions); diff != "" {
			t.Errorf("%s: conditions (-want +got):\n%s", name, diff)
		}
	}
}

func TestJestAndVitestReportTheSameRun(t *testing.T) {
	t.Parallel()
	_, j := squeeze(t, fixture(t, "run"), DefaultOptions())
	_, v := squeeze(t, fixture(t, "vitest"), DefaultOptions())
	if j.Tool != "jest" || v.Tool != "vitest" {
		t.Errorf("tools = %q, %q", j.Tool, v.Tool)
	}
	if diff := cmp.Diff(j.Tests, v.Tests); diff != "" {
		t.Errorf("tests (-jest +vitest):\n%s", diff)
	}
	if diff := cmp.Diff(j.Files, v.Files); diff != "" {
		t.Errorf("files (-jest +vitest):\n%s", diff)
	}
	names := func(r Report) []string {
		var out []string
		for _, f := range r.Failures {
			out = append(out, f.File+": "+f.Test)
		}
		for _, e := range r.FileErrors {
			out = append(out, e.File)
		}
		return out
	}
	want := []string{
		"test/cart.test.js: cart > applyDiscount > takes ten off with TEN",
		"test/cart.test.js: cart > summary lists tags",
		"test/cart.test.js: cart > parses a price",
		"test/format.test.js: summary snapshot",
		"test/broken.test.js",
		"test/hooks.test.js",
		"test/missing.test.js",
	}
	for _, r := range []Report{j, v} {
		if diff := cmp.Diff(want, names(r)); diff != "" {
			t.Errorf("%s failures (-want +got):\n%s", r.Tool, diff)
		}
		if r.Root != "/home/dev/shop" {
			t.Errorf("%s root = %q", r.Tool, r.Root)
		}
	}
}

func TestColorMatchesPlain(t *testing.T) {
	t.Parallel()
	plain, _ := squeeze(t, fixture(t, "run"), DefaultOptions())
	color, _ := squeeze(t, fixture(t, "color"), DefaultOptions())
	strip := func(s string) string {
		var out []string
		for _, l := range strings.Split(s, "\n") {
			if !strings.Contains(l, "(--full)") {
				out = append(out, l)
			}
		}
		return strings.Join(out, "\n")
	}
	if diff := cmp.Diff(strip(plain.Text), strip(color.Text)); diff != "" {
		t.Errorf("colored input differs from plain input (-plain +color):\n%s", diff)
	}
}

func TestStackFrames(t *testing.T) {
	t.Parallel()
	_, def := squeeze(t, fixture(t, "run"), DefaultOptions())
	_, full := squeeze(t, fixture(t, "run"), Options{Full: true})
	for i, f := range def.Failures {
		for _, l := range f.Output.Lines {
			if strings.Contains(l, "node_modules/") || strings.Contains(l, "node:internal") {
				t.Errorf("%s: internal frame kept: %q", f.Test, l)
			}
		}
		if got, want := len(f.Output.Lines)+f.Output.Hidden, len(full.Failures[i].Output.Lines); got != want {
			t.Errorf("%s: %d lines shown and %d hidden, %d with --full", f.Test, len(f.Output.Lines), f.Output.Hidden, want)
		}
		if full.Failures[i].Output.Hidden != 0 {
			t.Errorf("%s: --full hid %d lines", f.Test, full.Failures[i].Output.Hidden)
		}
	}
	frames := 0
	for _, l := range full.Failures[0].Output.Lines {
		if strings.Contains(l, "node_modules/jest-circus") {
			frames++
		}
	}
	if frames == 0 {
		t.Errorf("--full dropped the jest-circus frames: %q", full.Failures[0].Output.Lines)
	}
}

func TestFileErrorKeepsOnlySuiteBlocks(t *testing.T) {
	t.Parallel()
	input := `{"numTotalTests":1,"numFailedTests":1,"numRuntimeErrorTestSuites":1,"success":false,"testResults":[{"name":"/w/a.test.js","status":"failed",` +
		`"message":"  \u25cf a \u203a b\n\n    expected 1\n\n  \u25cf Test suite failed to run\n\n    afterAll broke\n\n      at x (a.test.js:3:9)\n",` +
		`"assertionResults":[{"ancestorTitles":["a"],"title":"b","fullName":"a b","status":"failed","failureMessages":["Error: expected 1"]}]}]}`
	r, data := squeeze(t, []byte(input), DefaultOptions())
	want := []FileError{{File: "a.test.js", Output: Lines{Lines: []string{"afterAll broke", "  at x (a.test.js:3:9)"}}}}
	if diff := cmp.Diff(want, data.FileErrors); diff != "" {
		t.Errorf("file errors (-want +got):\n%s", diff)
	}
	if strings.Count(r.Text, "expected 1") != 1 {
		t.Errorf("failed test block printed twice:\n%s", r.Text)
	}
	only := `{"numTotalTests":1,"testResults":[{"name":"/w/a.test.js","status":"failed","message":"  \u25cf a \u203a b\n\n    expected 1\n",` +
		`"assertionResults":[{"ancestorTitles":["a"],"title":"b","status":"failed","failureMessages":["expected 1"]}]}]}`
	if _, data := squeeze(t, []byte(only), DefaultOptions()); len(data.FileErrors) != 0 || data.Files.Failed != 1 {
		t.Errorf("a message with only failed test blocks made a file error: %+v", data.FileErrors)
	}
}

func TestLimits(t *testing.T) {
	t.Parallel()
	r, data := squeeze(t, fixture(t, "many"), Options{MaxTests: 5, MaxOutput: 2})
	if len(data.Failures) != 5 || data.FailuresOmitted != data.Tests.Failed-5 {
		t.Fatalf("failures = %d, omitted %d of %d", len(data.Failures), data.FailuresOmitted, data.Tests.Failed)
	}
	for _, f := range data.Failures {
		if len(f.Output.Lines) > 2 || f.Output.Omitted == 0 {
			t.Errorf("%s: %d lines, %d omitted", f.Test, len(f.Output.Lines), f.Output.Omitted)
		}
	}
	for _, want := range []string{"... 52 more failed tests (--max-tests 0)\n", "more lines (--max-output 0)\n"} {
		if !strings.Contains(r.Text, want) {
			t.Errorf("text lacks %q:\n%s", want, r.Text)
		}
	}
	_, all := squeeze(t, fixture(t, "many"), Options{})
	if len(all.Failures) != all.Tests.Failed || all.FailuresOmitted != 0 {
		t.Errorf("--max-tests 0 listed %d of %d", len(all.Failures), all.Tests.Failed)
	}
}

func TestAllListsNotRunAndPassing(t *testing.T) {
	t.Parallel()
	def, _ := squeeze(t, fixture(t, "run"), DefaultOptions())
	if !strings.Contains(def.Text, "... 1 skipped test, 1 todo test, 1 passing file (--all)\n") {
		t.Errorf("default text lacks the --all notice:\n%s", def.Text)
	}
	all, _ := squeeze(t, fixture(t, "run"), Options{All: true})
	for _, want := range []string{"pending test/cart.test.js: cart > applyDiscount > rejects expired codes\n", "todo test/cart.test.js: cart > applyDiscount > stacks two codes\n", "pass test/ok.test.js\n"} {
		if !strings.Contains(all.Text, want) {
			t.Errorf("--all text lacks %q", want)
		}
	}
}

func TestTrailingText(t *testing.T) {
	t.Parallel()
	doc := `{"numTotalTests":0,"testResults":[]}`
	for name, tt := range map[string]struct {
		trailer string
		want    []string
	}{
		"vitest notice":   {"JSON report written to /dev/stdout\n", []string{}},
		"none":            {"\n", []string{}},
		"stderr":          {"\nnpm ERR! Test failed.\nnpm ERR! code 1\n", []string{"npm ERR! Test failed.", "npm ERR! code 1"}},
		"notice and more": {"JSON report written to /dev/stdout\nnpm ERR! x\n", []string{"JSON report written to /dev/stdout", "npm ERR! x"}},
	} {
		r, data := squeeze(t, []byte(doc+tt.trailer), DefaultOptions())
		if diff := cmp.Diff(tt.want, data.Trailing.Lines); diff != "" {
			t.Errorf("%s: trailing (-want +got):\n%s", name, diff)
		}
		if len(tt.want) > 0 && !strings.Contains(r.Text, "output after the JSON:\n    "+tt.want[0]+"\n") {
			t.Errorf("%s: text lacks the trailing output:\n%s", name, r.Text)
		}
	}
}

func TestUnknownStatus(t *testing.T) {
	t.Parallel()
	input := `{"numTotalTests":2,"success":true,"testResults":[{"name":"/w/a.test.js","status":"passed","message":"","assertionResults":[` +
		`{"title":"x","status":"flaky"},{"title":"y","status":"flaky"}]}]}`
	r, data := squeeze(t, []byte(input), DefaultOptions())
	if diff := cmp.Diff(map[string]int{"flaky": 2}, data.UnknownStatus); diff != "" {
		t.Errorf("unknown status (-want +got):\n%s", diff)
	}
	if !strings.Contains(r.Text, "... 2 tests with an unknown status: flaky 2 (--raw-dir)\n") {
		t.Errorf("text lacks the unknown status notice:\n%s", r.Text)
	}
}

func TestStatusFollowsFailures(t *testing.T) {
	t.Parallel()
	input := `{"numTotalTests":1,"numFailedTests":1,"success":true,"testResults":[{"name":"/w/a.test.js","status":"failed","message":"",` +
		`"assertionResults":[{"title":"x","status":"failed","failureMessages":["boom"]}]}]}`
	r, data := squeeze(t, []byte(input), DefaultOptions())
	if data.Status != "fail" || !strings.HasPrefix(r.Text, "vitest: fail;") {
		t.Errorf("status = %q, text:\n%s", data.Status, r.Text)
	}
}

func TestRoot(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		files   []string
		message string
		want    string
	}{
		"node_modules above tests": {[]string{"/w/app/test/a.test.js"}, "at x (/w/app/node_modules/jest/a.js:1:1)", "/w/app"},
		"common directory":         {[]string{"/w/app/test/a.test.js", "/w/app/src/b.test.js"}, "", "/w/app"},
		"node_modules elsewhere":   {[]string{"/w/app/test/a.test.js"}, "at x (/other/node_modules/a.js:1:1)", "/w/app/test"},
		"relative":                 {[]string{"a.test.js"}, "", ""},
		"nothing shared":           {[]string{"/a/x.test.js", "/b/y.test.js"}, "", ""},
	}
	for name, tt := range tests {
		var files []TestFile
		for _, f := range tt.files {
			files = append(files, TestFile{Name: f, Message: tt.message})
		}
		if got := root(&Output{}, files); got != tt.want {
			t.Errorf("%s: root() = %q, want %q", name, got, tt.want)
		}
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := []byte(`{"numTotalTests":1,"numFailedTests":1,"success":false,"openHandles":[{"name":"E","message":"m\nfail forged","stack":"E\n    at x\nfail forged"}],` +
		`"testResults":[{"name":"/w/a.test.js\nfail forged","status":"failed","message":"\u001b[31mfail forged\u001b[39m\nfail forged",` +
		`"assertionResults":[{"ancestorTitles":["d\nfail forged"],"title":"t\u001b[2K\nfail forged","status":"failed","failureMessages":["boom\nfail forged\r\n\u001b]0;x\u0007fail forged"]}]}]}` +
		"\nfail forged\n")
	r, _ := squeeze(t, input, DefaultOptions())
	formattest.CheckText(t, r.Text)
	for _, line := range strings.Split(r.Text, "\n") {
		if strings.HasPrefix(line, "fail forged") || strings.HasPrefix(line, "  fail forged") {
			t.Errorf("input forged a line: %q\n%s", line, r.Text)
		}
	}
}

func TestDataPayload(t *testing.T) {
	t.Parallel()
	r, _ := squeeze(t, fixture(t, "vitest-pass"), DefaultOptions())
	out, err := core.Envelope("jest", 10, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ReplaceAll(string(out), `"run_error":null`, ""), "null") {
		t.Errorf("envelope has a null collection: %s", out)
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(`{"numTotalTests":1,"testResults":[{"name":"/a","message":"\u25cf x","assertionResults":[{"status":"failed","failureMessages":null},{"status":"?"}]}],"runExecError":{"stack":""}}x`))
	f.Fuzz(func(t *testing.T, input []byte) {
		out, err := Parse(input)
		if err != nil {
			return
		}
		failed := 0
		for _, file := range *out.TestResults {
			for _, a := range file.AssertionResults {
				if a.Status == "failed" {
					failed++
				}
			}
		}
		for _, o := range []Options{DefaultOptions(), {MaxTests: 1, MaxOutput: 1}, {Full: true, All: true}} {
			r := Build(out, o)
			res := core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			formattest.CheckNotices(t, res.Text)
			if len(r.Failures)+r.FailuresOmitted != failed {
				t.Fatalf("%d failures listed and %d omitted, %d in the input", len(r.Failures), r.FailuresOmitted, failed)
			}
			if o.MaxTests > 0 && len(r.Failures) > o.MaxTests {
				t.Fatalf("%d failures over the limit %d", len(r.Failures), o.MaxTests)
			}
			if r.FailuresOmitted > 0 && !strings.Contains(res.Text, "(--max-tests 0)") {
				t.Fatalf("failures cut without a notice:\n%s", res.Text)
			}
			if (failed > 0 || len(r.FileErrors) > 0) && r.Status != "fail" {
				t.Fatalf("status %q with %d failed tests and %d file errors", r.Status, failed, len(r.FileErrors))
			}
			if !strings.HasPrefix(res.Text, r.Tool+": "+r.Status+";") {
				t.Fatalf("header does not lead with the tool and status:\n%s", res.Text)
			}
			if _, err := core.Envelope("jest", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

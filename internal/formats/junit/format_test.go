package junit

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
)

const fixtureDir = "testdata"

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name, "input.xml"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustReport(t *testing.T, input []byte, o options) report {
	t.Helper()
	r, err := squeeze(input, o)
	if err != nil {
		t.Fatal(err)
	}
	return r.Data.(report)
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
		"testsuites":         {`<testsuites></testsuites>`, true},
		"bare testsuite":     {`<?xml version="1.0"?><testsuite name="a" tests="0"/>`, true},
		"comment and bom":    {"\xef\xbb\xbf\n<!-- report -->\n<!DOCTYPE testsuites>\n<testsuites>", true},
		"namespace prefix":   {`<j:testsuite xmlns:j="urn:x"/>`, true},
		"other root":         {`<?xml version="1.0"?><project><testsuite/></project>`, false},
		"checkstyle":         {`<?xml version="1.0"?><checkstyle version="10.0"><file name="a.java"/></checkstyle>`, false},
		"html":               {`<!doctype html><html><body>testsuite</body></html>`, false},
		"text before":        {`report: <testsuites/>`, false},
		"broken declaration": {`<?xml version="1.0"`, false},
		"sarif":              {`{"version":"2.1.0","runs":[]}`, false},
		"gotest text":        {"=== RUN   TestA\n--- PASS: TestA (0.00s)\nPASS\nok  \texample.com/a\t0.1s\n", false},
		"pytest text":        {"============================= test session starts ==============================\n", false},
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
		"truncated":   `<testsuites><testsuite name="a">`,
		"other root":  `<checkstyle/>`,
		"not xml":     `{"tests":1}`,
		"empty":       ``,
		"text before": `0<testsuite></testsuite>`,
	} {
		if _, err := squeeze([]byte(input), defaultOptions()); err == nil {
			t.Errorf("%s: Squeeze() returned no error", name)
		}
	}
}

func TestCounts(t *testing.T) {
	t.Parallel()
	type counts struct{ Suites, Tests, Passed, Failures, Errors, Skipped, Flaky, Cases int }
	tests := map[string]counts{
		"pytest":          {1, 9, 2, 4, 2, 2, 0, 6},
		"gotestsum":       {1, 7, 1, 5, 0, 1, 0, 5},
		"go-junit-report": {1, 7, 1, 5, 0, 1, 0, 5},
		"checkov":         {1, 9, 7, 2, 0, 0, 0, 2},
		"jest":            {1, 4, 2, 1, 0, 1, 0, 1},
		"surefire":        {1, 6, 2, 2, 1, 1, 1, 3},
		"ant-nested":      {3, 3, 2, 0, 0, 1, 0, 0},
	}
	for name, want := range tests {
		r := mustReport(t, fixture(t, name), defaultOptions())
		got := counts{r.Suites, r.Tests, r.Passed, r.Failures, r.Errors, r.Skipped, r.Flaky, len(r.Cases)}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%s: counts (-want +got):\n%s", name, diff)
		}
	}
}

func TestMismatchWithTheAttributes(t *testing.T) {
	t.Parallel()
	r := mustReport(t, fixture(t, "jest"), defaultOptions())
	if diff := cmp.Diff([]mismatch{{Count: "tests", Counted: 4, Attribute: 3}}, r.Mismatches); diff != "" {
		t.Errorf("jest mismatches (-want +got):\n%s", diff)
	}
	for _, name := range []string{"pytest", "gotestsum", "go-junit-report", "checkov", "surefire", "ant-nested"} {
		if r := mustReport(t, fixture(t, name), defaultOptions()); len(r.Mismatches) != 0 {
			t.Errorf("%s: mismatches %+v", name, r.Mismatches)
		}
	}
	none := mustReport(t, []byte(`<testsuites><testsuite name="a"><testcase name="t"/></testsuite></testsuites>`), defaultOptions())
	if none.Attributes.Tests != nil || len(none.Mismatches) != 0 {
		t.Errorf("attributes %+v, mismatches %+v", none.Attributes, none.Mismatches)
	}
	attr := mustReport(t, []byte(`<testsuite name="a" tests="2" failures="1"><testcase name="t"/></testsuite>`), defaultOptions())
	if diff := cmp.Diff([]mismatch{{Count: "tests", Counted: 1, Attribute: 2}, {Count: "failures", Counted: 0, Attribute: 1}}, attr.Mismatches); diff != "" {
		t.Errorf("bare suite mismatches (-want +got):\n%s", diff)
	}
	if got := conditions(attr); !cmp.Equal(got, []string{"fail"}) {
		t.Errorf("failure in the attributes only: conditions = %v, want [fail]", got)
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"pytest":     {"error", "fail", "skip"},
		"gotestsum":  {"fail", "skip"},
		"checkov":    {"fail"},
		"surefire":   {"error", "fail", "flaky", "skip"},
		"ant-nested": {"skip"},
	}
	for name, want := range tests {
		r, err := squeeze(fixture(t, name), defaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, r.Conditions); diff != "" {
			t.Errorf("%s: conditions (-want +got):\n%s", name, diff)
		}
	}
}

func TestCases(t *testing.T) {
	t.Parallel()
	r := mustReport(t, fixture(t, "pytest-logging"), defaultOptions())
	var got []string
	for _, c := range r.Cases {
		got = append(got, fmt.Sprintf("%s %s %s %d/%d/%d", c.Kind, c.Classname, c.Name, len(c.Text.Lines), len(c.Stdout.Lines), len(c.Stderr.Lines)))
	}
	want := []string{
		"failure test_cart test_total_two_items 7/1/1",
		"error test_cart test_checkout_uses_db 5/0/0",
		"failure test_cart test_strict_xpass 0/0/0",
		"failure test_cart.TestDiscount test_negative 6/0/0",
		"failure test_cart test_order_export 7/0/0",
		"error test_cart test_order_export 6/0/0",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("cases (-want +got):\n%s", diff)
	}
	jest := mustReport(t, fixture(t, "jest"), defaultOptions())
	if c := jest.Cases[0]; c.Message != "Error: expect(received).toBe(expected) // Object.is equality" || c.Text.Lines[0] != "Expected: 4" {
		t.Errorf("message from the text: %q, first line %q", c.Message, c.Text.Lines[0])
	}
	gojunit := mustReport(t, fixture(t, "go-junit-report"), defaultOptions())
	if diff := cmp.Diff([]string{"example.com/shop"}, []string{gojunit.SuiteOutput[0].Suite}); diff != "" || !strings.HasPrefix(gojunit.SuiteOutput[0].Stdout.Lines[0], "panic: ") {
		t.Errorf("suite output %+v", gojunit.SuiteOutput)
	}
}

func TestLimits(t *testing.T) {
	t.Parallel()
	r, err := squeeze(fixture(t, "pytest-logging"), options{MaxTests: 2, MaxOutput: 3})
	if err != nil {
		t.Fatal(err)
	}
	data := r.Data.(report)
	first := data.Cases[0]
	if len(data.Cases) != 2 || data.TestsOmitted != 4 || len(first.Text.Lines) != 3 || first.Text.Omitted != 4 {
		t.Errorf("cases %d, omitted %d, lines %d, lines omitted %d", len(data.Cases), data.TestsOmitted, len(first.Text.Lines), first.Text.Omitted)
	}
	for _, want := range []string{"\n  ... 4 more lines (--max-output 0)\n", "\n... 4 more failed tests (--max-tests 0)\n"} {
		if !strings.Contains(r.Text, want) {
			t.Errorf("text lacks %q:\n%s", want, r.Text)
		}
	}
	all := mustReport(t, fixture(t, "pytest-logging"), options{})
	if len(all.Cases) != 6 || all.Cases[0].Text.Omitted != 0 {
		t.Errorf("with no limits: %d cases, %d lines omitted", len(all.Cases), all.Cases[0].Text.Omitted)
	}
}

func TestUnnamedCaseAndDeclaredEncoding(t *testing.T) {
	t.Parallel()
	input := []byte(`<?xml version="1.0" encoding="ISO-8859-1"?><testsuite><testcase classname="c" file="a.py" line="7"><error/></testcase><system-out>boom</system-out></testsuite>`)
	r, err := squeeze(input, defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	want := "junit: 1 test in 1 suite: 1 error\nerror c (no name): no message\nsuite without a name stdout:\n  boom\n"
	if diff := cmp.Diff(want, r.Text); diff != "" {
		t.Errorf("text (-want +got):\n%s", diff)
	}
	if c := r.Data.(report).Cases[0]; c.File != "a.py" || c.Line != 7 {
		t.Errorf("file %q, line %d", c.File, c.Line)
	}
}

func TestEncodings(t *testing.T) {
	t.Parallel()
	tests := map[string]struct{ input, want string }{
		"latin1 declared as ISO-8859-1": {
			"<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><testsuite><testcase classname=\"pkg\" name=\"caf\xe9\"><failure message=\"r\xe9sum\xe9\"/></testcase></testsuite>",
			"fail pkg caf\u00e9: r\u00e9sum\u00e9\n",
		},
		"latin1 declared as latin1": {
			"<?xml version=\"1.0\" encoding=\"latin1\"?><testsuite><testcase classname=\"c\" name=\"\xfcber\"><failure message=\"m\"/></testcase></testsuite>",
			"fail c \u00fcber: m\n",
		},
		"US-ASCII": {
			`<?xml version="1.0" encoding="US-ASCII"?><testsuite><testcase classname="c" name="t"><failure message="m"/></testcase></testsuite>`,
			"fail c t: m\n",
		},
	}
	for name, tt := range tests {
		r, err := squeeze([]byte(tt.input), defaultOptions())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if diff := cmp.Diff("junit: 1 test in 1 suite: 1 failure\n"+tt.want, r.Text); diff != "" {
			t.Errorf("%s: text (-want +got):\n%s", name, diff)
		}
	}
	utf16 := []byte(`<?xml version="1.0" encoding="UTF-16"?><testsuite name="s"/>`)
	if _, err := squeeze(utf16, defaultOptions()); err == nil || !strings.Contains(err.Error(), `unsupported encoding "UTF-16"`) {
		t.Errorf("UTF-16: error %v", err)
	}
	if !(Format{}).Detect(utf16) {
		t.Error("a report in an unsupported encoding is not detected, so its error would name another format")
	}
}

func TestLongMessageKeptWholeInData(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 400)
	r, err := squeeze([]byte(`<testsuite name="s"><testcase classname="c" name="t"><failure message="`+long+`"/></testcase></testsuite>`), defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Data.(report).Cases[0].Message != long || strings.Contains(r.Text, long) || !strings.Contains(r.Text, "...") {
		t.Errorf("text:\n%s", r.Text)
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := []byte("<testsuites><testsuite name=\"s&#10;fail forged\">" +
		"<testcase classname=\"c&#10;fail forged\" name=\"t&#x2028;fail forged\"><failure message=\"m&#10;fail forged&#x85;\">line&#x9;one\nfail forged&#xd;\n&#x9b;31m</failure>" +
		"<system-out>fail forged</system-out></testcase><system-err>fail forged</system-err></testsuite></testsuites>")
	for _, o := range []options{defaultOptions(), {MaxOutput: 1}} {
		r, err := squeeze(input, o)
		if err != nil {
			t.Fatal(err)
		}
		formattest.CheckText(t, r.Text)
		for _, line := range strings.Split(r.Text, "\n") {
			if strings.HasPrefix(line, "fail forged") {
				t.Errorf("input forged a line: %q\n%s", line, r.Text)
			}
		}
	}
	if _, err := squeeze([]byte("<testsuite name=\"a\x1b[31m\"/>"), defaultOptions()); err == nil {
		t.Error("a raw escape byte, which XML 1.0 does not allow, was accepted")
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(`<testsuites tests="x" failures="-1"><testcase name="a"><error/><failure/><skipped/></testcase><testsuite><testsuite><testcase line="-3"><flakyError/></testcase></testsuite></testsuite></testsuites>`))
	f.Fuzz(func(t *testing.T, input []byte) {
		formattest.CheckStable(t, func() (core.Result, error) { return squeeze(input, defaultOptions()) })
		root, err := parse(input)
		if err != nil {
			return
		}
		if !detect(core.Head(input)) {
			t.Fatalf("parsed but not detected: %q", input)
		}
		for _, o := range []options{defaultOptions(), {MaxTests: 1, MaxOutput: 1}} {
			r := build(root, o)
			res := core.Result{Text: render(r), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			if len(r.Cases)+r.TestsOmitted < r.Failures+r.Errors {
				t.Fatalf("%d cases shown and %d omitted, %d failures and %d errors", len(r.Cases), r.TestsOmitted, r.Failures, r.Errors)
			}
			if r.Passed < 0 || r.Passed+r.Failures+r.Errors+r.Skipped < r.Tests {
				t.Fatalf("passed %d, failures %d, errors %d, skipped %d, tests %d", r.Passed, r.Failures, r.Errors, r.Skipped, r.Tests)
			}
			if o.MaxTests > 0 && len(r.Cases) > o.MaxTests {
				t.Fatalf("%d cases over the limit %d", len(r.Cases), o.MaxTests)
			}
			if r.TestsOmitted > 0 && !strings.Contains(res.Text, "(--max-tests 0)") {
				t.Fatalf("cases cut without a notice:\n%s", res.Text)
			}
			for _, c := range r.Cases {
				if o.MaxOutput > 0 && len(c.Text.Lines) > o.MaxOutput {
					t.Fatalf("%d lines over the limit %d", len(c.Text.Lines), o.MaxOutput)
				}
				if c.Text.Omitted > 0 && !strings.Contains(res.Text, "(--max-output 0)") {
					t.Fatalf("lines cut without a notice:\n%s", res.Text)
				}
			}
			if (r.Failures+r.Errors > 0) != slices.Contains(res.Conditions, "fail") && value(r.Attributes.Failures)+value(r.Attributes.Errors) <= 0 {
				t.Fatalf("fail condition %v with %d failures and %d errors", res.Conditions, r.Failures, r.Errors)
			}
			if _, err := core.Envelope("junit", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

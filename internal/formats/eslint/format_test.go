package eslint

import (
	"fmt"
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
		"json":              {`[{"filePath":"/a.js","messages":[],"errorCount":0}]`, true},
		"pretty json":       {"[\n  {\n    \"filePath\": \"/a.js\",\n    \"messages\": [],\n    \"errorCount\": 0\n  }\n]", true},
		"with metadata":     {`{"results":[{"filePath":"/a.js","messages":[],"errorCount":0}],"metadata":{}}`, true},
		"no error count":    {`[{"filePath":"/a.js","messages":[]}]`, false},
		"file path later":   {`[{"messages":[],"filePath":"/a.js","errorCount":0}]`, false},
		"empty array":       {`[]`, false},
		"tflint":            {`{"issues":[],"errors":[]}`, false},
		"checkov list":      {`[{"check_type":"terraform","results":{}}]`, false},
		"terraform outputs": {`{"bucket":{"sensitive":false,"type":"string","value":"x"}}`, false},
		"plain text":        {"/a.js\n  1:1  error  x  no-var", false},
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
		"truncated":         `[{"filePath":`,
		"no messages":       `[{"filePath":"/a.js","errorCount":0}]`,
		"no file path":      `[{"messages":[],"errorCount":0}]`,
		"metadata only":     `{"metadata":{"cwd":"/"}}`,
		"wrong type":        `{"results":{}}`,
		"messages not list": `[{"filePath":"/a.js","messages":{}}]`,
	} {
		if _, err := Squeeze([]byte(input), DefaultOptions()); err == nil {
			t.Errorf("%s: Squeeze() returned no error", name)
		}
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"problems": {"error", "fail", "fixable", "warning"},
		"metadata": {"error", "fail", "fixable", "warning"},
		"clean":    {},
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
	r := report(t, []byte(`[{"filePath":"/a.js","messages":[{"ruleId":"no-var","severity":1,"message":"w","line":1,"column":1}],"errorCount":0,"warningCount":1}]`), DefaultOptions())
	if got := conditions(r); !cmp.Equal(got, []string{"warning"}) {
		t.Errorf("warnings only: conditions = %v, want [warning]: ESLint exits 0 without --max-warnings", got)
	}
}

func TestProblems(t *testing.T) {
	t.Parallel()
	r := report(t, fixture(t, "problems"), DefaultOptions())
	if r.Root != "/home/dev/shop" || r.Files != 5 || r.FilesWithProblem != 3 || r.Errors != 7 || r.Warnings != 5 || r.Fatal != 1 || r.Fixable != 6 || r.Suppressed != 1 {
		t.Errorf("counts = %+v", r)
	}
	want := []string{"error Parsing error: Unexpected keyword 'return' src/broken/parse.js:2:3"}
	var got []string
	for _, d := range r.ParseErrors {
		got = append(got, fmt.Sprintf("%s %s %s:%d:%d", d.Severity, d.Summary, d.File, d.Line, d.Column))
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("parse errors (-want +got):\n%s", diff)
	}
	var directive *Problem
	for i, p := range r.Problems {
		if p.Rule == "" {
			directive = &r.Problems[i]
		}
	}
	if directive == nil || directive.Severity != "warning" || !directive.Fixable || directive.File != "src/lib/format.js" {
		t.Errorf("unused directive problem = %+v", directive)
	}
	if len(r.Problems)+len(r.ParseErrors) != r.Errors+r.Warnings {
		t.Errorf("%d problems listed, %d counted", len(r.Problems)+len(r.ParseErrors), r.Errors+r.Warnings)
	}
}

func TestRoot(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		cwd   string
		files []string
		want  string
	}{
		"cwd wins":         {"/w/app/", []string{"/w/app/src/a.js"}, "/w/app"},
		"common directory": {"", []string{"/w/app/src/a.js", "/w/app/lib/b.js", "/w/app/src/x/c.js"}, "/w/app"},
		"one file":         {"", []string{"/w/app/src/a.js"}, "/w/app/src"},
		"sibling prefix":   {"", []string{"/w/app/a.js", "/w/application/b.js"}, "/w"},
		"nothing shared":   {"", []string{"/a/x.js", "/b/y.js"}, ""},
		"relative paths":   {"", []string{"a.js", "b.js"}, ""},
	}
	for name, tt := range tests {
		out := &Output{Cwd: tt.cwd}
		for _, f := range tt.files {
			out.Results = append(out.Results, Result{FilePath: f, Messages: &[]Message{}})
		}
		if got := root(out); got != tt.want {
			t.Errorf("%s: root() = %q, want %q", name, got, tt.want)
		}
	}
}

func TestLimitNamesTheOmittedRules(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "many"), Options{MaxProblems: 5})
	if err != nil {
		t.Fatal(err)
	}
	data := r.Data.(Report)
	total := data.Errors + data.Warnings
	if len(data.Problems) != 5 || data.ProblemsOmitted != total-5 {
		t.Fatalf("problems = %d, omitted %d of %d", len(data.Problems), data.ProblemsOmitted, total)
	}
	sum := 0
	for _, c := range data.OmittedByRule {
		sum += c.Count
	}
	if sum != data.ProblemsOmitted || !strings.Contains(r.Text, "(--max-problems 0)\n") {
		t.Errorf("omitted by rule %v sums to %d, omitted %d, text:\n%s", data.OmittedByRule, sum, data.ProblemsOmitted, r.Text)
	}
}

func TestParseErrorsAreNeverCut(t *testing.T) {
	t.Parallel()
	input := []byte(`[{"filePath":"/w/a.js","messages":[{"ruleId":null,"fatal":true,"severity":2,"message":"Parsing error: x","line":1,"column":1}],"errorCount":1,"fatalErrorCount":1},` +
		`{"filePath":"/w/b.js","messages":[{"ruleId":"r","severity":2,"message":"m1","line":1,"column":1},{"ruleId":"r","severity":2,"message":"m2","line":2,"column":1}],"errorCount":2}]`)
	r, err := Squeeze(input, Options{MaxProblems: 1})
	if err != nil {
		t.Fatal(err)
	}
	want := "eslint: 3 problems (3 errors, 0 warnings) in 2 of 2 files; 1 file not parsed\n" +
		"error: Parsing error: x (a.js:1:1)\n" +
		"error: m1 (b.js:1:1, r)\n" +
		"... 1 more problem, r 1 (--max-problems 0)\n"
	if r.Text != want {
		t.Errorf("text:\n%s\nwant:\n%s", r.Text, want)
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := []byte(`[{"filePath":"/w/a.js\nerror: forged","messages":[{"ruleId":"r\nerror: forged","severity":2,"message":"m\u001b[31m\nerror: forged","line":1,"column":1},` +
		`{"ruleId":null,"fatal":true,"severity":2,"message":"p\nerror: forged","line":1,"column":1}],"errorCount":2,"fatalErrorCount":1}]`)
	r, err := Squeeze(input, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	formattest.CheckText(t, r.Text)
	for _, line := range strings.Split(r.Text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "error: forged") {
			t.Errorf("input forged a line: %q\n%s", line, r.Text)
		}
	}
}

func TestDataPayload(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "clean"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	out, err := core.Envelope("eslint", 10, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "null") {
		t.Errorf("envelope has a null collection: %s", out)
	}
	if strings.Contains(string(fixture(t, "problems")), `"source"`) {
		r, _ := Squeeze(fixture(t, "problems"), DefaultOptions())
		if strings.Contains(r.Text, "export function") {
			t.Errorf("source text reached the output:\n%s", r.Text)
		}
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(`[{"filePath":"/a","messages":[{"ruleId":null,"fatal":true},{"severity":7,"fix":{}}],"errorCount":-1}]`))
	f.Fuzz(func(t *testing.T, input []byte) {
		out, err := Parse(input)
		if err != nil {
			return
		}
		messages := 0
		for _, res := range out.Results {
			messages += len(*res.Messages)
		}
		for _, o := range []Options{DefaultOptions(), {MaxProblems: 1}} {
			r := Build(out, o)
			res := core.Result{Text: Text(r), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			shown := len(r.ParseErrors)
			for _, d := range r.ParseErrors {
				shown += max(d.Count, 1) - 1
			}
			for _, p := range r.Problems {
				shown += max(p.Count, 1)
			}
			if shown+r.ProblemsOmitted != messages {
				t.Fatalf("%d messages shown and %d omitted, %d in the input", shown, r.ProblemsOmitted, messages)
			}
			if o.MaxProblems > 0 && len(r.Problems) > o.MaxProblems {
				t.Fatalf("%d lines over the limit %d", len(r.Problems), o.MaxProblems)
			}
			if r.ProblemsOmitted > 0 && !strings.Contains(res.Text, "(--max-problems 0)") {
				t.Fatalf("problems cut without a notice:\n%s", res.Text)
			}
			if _, err := core.Envelope("eslint", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

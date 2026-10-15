package golangci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
	"github.com/cauebraz/squeeze/internal/tfdiag"
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
		"issues":         {`{"Issues":[{"FromLinter":"errcheck","Text":"x"}],"Report":{}}`, true},
		"clean":          {`{"Issues":[],"Report":{"Linters":[]}}`, true},
		"pretty":         {"{\n  \"Issues\": [\n    {\n      \"FromLinter\": \"govet\"\n    }\n  ]\n}", true},
		"no linter key":  {`{"Issues":[{}]}`, false},
		"issues later":   {`{"Report":{},"Issues":[]}`, false},
		"tflint":         {`{"issues":[],"errors":[]}`, false},
		"eslint":         {`[{"filePath":"/a.js","messages":[],"errorCount":0}]`, false},
		"sarif":          {`{"version":"2.1.0","runs":[]}`, false},
		"trivy":          {`{"SchemaVersion":2,"Results":[]}`, false},
		"plain text":     {"cart/cart.go:15:9: Error return value is not checked (errcheck)", false},
		"stats only":     {"0 issues.\n", false},
		"issues object":  {`{"Issues":{}}`, false},
		"leading spaces": {" \n\t{\"Issues\":[],\"Report\":{}}", true},
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
		"truncated":       `{"Issues":[`,
		"no issues":       `{"Report":{}}`,
		"null issues":     `{"Issues":null}`,
		"wrong type":      `{"Issues":{}}`,
		"line not number": `{"Issues":[{"Pos":{"Line":"1"}}]}`,
		"array":           `[]`,
	} {
		if _, err := Squeeze([]byte(input), DefaultOptions()); err == nil {
			t.Errorf("%s: Squeeze() returned no error", name)
		}
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"issues":         {"error", "fail", "fixable", "warning"},
		"severity":       {"error", "fail", "fixable", "warning"},
		"typecheck":      {"error", "fail"},
		"report-error":   {"error", "fail", "warning"},
		"v1-replacement": {"error", "fail", "fixable", "warning"},
		"clean":          {},
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
	r := report(t, []byte(`{"Issues":[{"FromLinter":"unused","Text":"x","Severity":"info"}]}`), DefaultOptions())
	if got := conditions(r); !cmp.Equal(got, []string{"fail"}) {
		t.Errorf("info issue: conditions = %v, want [fail]: golangci-lint exits 1 on any issue", got)
	}
	r = report(t, []byte(`{"Issues":[],"Report":{"Error":"typechecking error: x"}}`), DefaultOptions())
	if got := conditions(r); !cmp.Equal(got, []string{"error", "fail"}) {
		t.Errorf("report error: conditions = %v, want [error fail]: golangci-lint exits 7", got)
	}
}

func TestIssues(t *testing.T) {
	t.Parallel()
	r := report(t, fixture(t, "issues"), DefaultOptions())
	if r.Total != 12 || r.Errors != 12 || r.Warnings != 0 || r.Fixable != 1 || r.RunWarnings != 1 || r.RunErrors != 0 || r.IssuesOmitted != 0 {
		t.Errorf("counts = %+v", r)
	}
	want := []core.Count{{Name: "errcheck", Count: 5}, {Name: "staticcheck", Count: 3}, {Name: "ineffassign", Count: 2}, {Name: "govet", Count: 1}, {Name: "unused", Count: 1}}
	if diff := cmp.Diff(want, r.Linters); diff != "" {
		t.Errorf("linters (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"errcheck", "govet", "ineffassign", "staticcheck", "typecheck", "unused"}, r.EnabledLinters); diff != "" {
		t.Errorf("enabled linters (-want +got):\n%s", diff)
	}
	sum := 0
	for _, p := range r.Issues {
		sum += max(p.Count, 1)
	}
	if sum != r.Total {
		t.Errorf("%d issues listed, %d counted", sum, r.Total)
	}
}

func TestSeverities(t *testing.T) {
	t.Parallel()
	r := report(t, fixture(t, "severity"), DefaultOptions())
	want := []core.Count{{Name: "warning", Count: 6}, {Name: "error", Count: 5}, {Name: "info", Count: 1}}
	if diff := cmp.Diff(want, r.Severities); diff != "" {
		t.Errorf("severities (-want +got):\n%s", diff)
	}
	if !strings.HasPrefix(Text(r), "golangci-lint: 12 issues (5 errors, 6 warnings, 1 info) from ") {
		t.Errorf("header:\n%s", Text(r))
	}
	r = report(t, []byte(`{"Issues":[{"FromLinter":"a","Text":"x","Severity":" WARNING\n"},{"FromLinter":"a","Text":"y"}]}`), DefaultOptions())
	if r.Warnings != 1 || r.Errors != 1 {
		t.Errorf("severity case and empty severity: %+v", r.Severities)
	}
}

func TestTypecheckSplitsCompilerErrors(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		text string
		want []string
	}{
		"go list errors": {
			": # example.com/shop/broken\nbroken/broken.go:4:14: cannot use x\nbroken/broken.go:5:13: undefined: missing",
			[]string{"error: cannot use x (broken/broken.go:4:14, typecheck)", "error: undefined: missing (broken/broken.go:5:13, typecheck)"},
		},
		"no column and continuation": {
			"a.go:3: first\n\tmore detail\nb.go:1:2: second",
			[]string{"error: first more detail (a.go:3, typecheck)", "error: second (b.go:1:2, typecheck)"},
		},
		"one message": {
			": found packages mixed (a.go) and other (b.go) in mixed",
			[]string{"error: found packages mixed (a.go) and other (b.go) in mixed (mixed/a.go:1, typecheck)"},
		},
		"nested location stays one issue": {
			"could not import example.org/nope/v9 (miss/miss.go:3:8: no required module provides package example.org/nope/v9; to add it:\n\tgo get example.org/nope/v9)",
			[]string{"error: could not import example.org/nope/v9 (miss/miss.go:3:8: no required module provides package example.org/nope/v9; to add it: go get example.org/nope/v9) (mixed/a.go:1, typecheck)"},
		},
		"text before compiler errors is kept": {
			"loading failed\nc.go:1:1: bad",
			[]string{"error: loading failed (mixed/a.go:1, typecheck)", "error: bad (c.go:1:1, typecheck)"},
		},
		"header only": {
			"# example.com/x",
			[]string{"error: # example.com/x (mixed/a.go:1, typecheck)"},
		},
	}
	for name, tt := range tests {
		base := tfdiag.Diagnostic{Severity: "error", Address: "typecheck", File: "mixed/a.go", Line: 1}
		var got []string
		for _, d := range expand(tt.text, base) {
			var b strings.Builder
			tfdiag.Write(&b, d, tfdiag.WriteOptions{})
			got = append(got, strings.TrimSuffix(b.String(), "\n"))
		}
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("%s: expand() (-want +got):\n%s", name, diff)
		}
	}
}

func TestTypecheckIsNeverCut(t *testing.T) {
	t.Parallel()
	input := []byte(`{"Issues":[{"FromLinter":"errcheck","Text":"e1","Pos":{"Filename":"a.go","Line":1}},{"FromLinter":"errcheck","Text":"e2","Pos":{"Filename":"a.go","Line":2}},` +
		`{"FromLinter":"typecheck","Text":": # p\nz.go:1:1: undefined: x\nz.go:2:1: undefined: y","Pos":{"Filename":"z.go","Line":1}}],"Report":{"Error":"typechecking error: boom"}}`)
	r, err := Squeeze(input, Options{MaxIssues: 1})
	if err != nil {
		t.Fatal(err)
	}
	want := "golangci-lint: 4 issues (4 errors) from errcheck 2, typecheck 2; 1 run error\n" +
		"error: typechecking error: boom\n" +
		"error: undefined: x (z.go:1:1, typecheck)\n" +
		"error: undefined: y (z.go:2:1, typecheck)\n" +
		"error: e1 (a.go:1, errcheck)\n" +
		"... 1 more issue, errcheck 1 (--max-issues 0)\n"
	if diff := cmp.Diff(want, r.Text); diff != "" {
		t.Errorf("text (-want +got):\n%s", diff)
	}
}

func TestLimitNamesTheOmittedLinters(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "many"), Options{MaxIssues: 5})
	if err != nil {
		t.Fatal(err)
	}
	data := r.Data.(Report)
	if len(data.Issues) != 5 || data.IssuesOmitted != data.Total-5 {
		t.Fatalf("issues = %d, omitted %d of %d", len(data.Issues), data.IssuesOmitted, data.Total)
	}
	sum := 0
	for _, c := range data.OmittedByLinter {
		sum += c.Count
	}
	if sum != data.IssuesOmitted || !strings.Contains(r.Text, "(--max-issues 0)\n") {
		t.Errorf("omitted by linter %v sums to %d, omitted %d, text:\n%s", data.OmittedByLinter, sum, data.IssuesOmitted, r.Text)
	}
}

func TestTrailingText(t *testing.T) {
	t.Parallel()
	input := []byte("{\"Issues\":[],\"Report\":{}}\n0 issues.\n3 issues:\n* errcheck: 3\nlevel=error msg=\"boom\"\nsomething else\n")
	r := report(t, input, DefaultOptions())
	var got []string
	for _, d := range r.Run {
		got = append(got, d.Severity+" "+d.Summary)
	}
	if diff := cmp.Diff([]string{`error level=error msg="boom"`, "warning something else"}, got); diff != "" {
		t.Errorf("run diagnostics (-want +got):\n%s", diff)
	}
	if r.RunErrors != 1 || r.RunWarnings != 1 {
		t.Errorf("run counts = %d, %d", r.RunErrors, r.RunWarnings)
	}
}

func TestFoldKeepsFixable(t *testing.T) {
	t.Parallel()
	got := fold([]Problem{
		{Diagnostic: tfdiag.Diagnostic{Severity: "error", Summary: "s", Address: "a"}, Linter: "a"},
		{Diagnostic: tfdiag.Diagnostic{Severity: "error", Summary: "s", Address: "a"}, Linter: "a", Fixable: true},
	})
	if len(got) != 1 || got[0].Count != 2 || !got[0].Fixable {
		t.Errorf("fold() = %+v", got)
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := []byte(`{"Issues":[{"FromLinter":"lint\nerror: forged","Text":"m\u001b[31m\nerror: forged","Severity":"warning\nerror: forged","Pos":{"Filename":"a.go\nerror: forged","Line":1}},` +
		`{"FromLinter":"typecheck","Text":"a.go:1:1: x\nerror: forged\n\u001b[2Jb.go:2:2: y\u001b[0m"}],"Report":{"Error":"e\nerror: forged","Warnings":[{"Tag":"t\nerror: forged","Text":"w\nerror: forged"}]}}` + "\nerror: forged")
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
	out, err := core.Envelope("golangci", 10, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "null") {
		t.Errorf("envelope has a null collection: %s", out)
	}
	r, err = Squeeze(fixture(t, "issues"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.Text, "strings.Join(c.Items") || strings.Contains(r.Text, "c3RyaW5ncy5SZXBsYWNlQWxs") {
		t.Errorf("source lines or fix edits reached the output:\n%s", r.Text)
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(`{"Issues":[{"FromLinter":"typecheck","Text":"a.go:1:1: x\n\ty\n# p\nb.go:99999999999999999999:1: z"},{"FromLinter":"","Severity":"x","SuggestedFixes":[{}],"Replacement":{}}],"Report":{"Error":"e","Warnings":[{}]}}` + "\n1 issues:\nlevel=fatal x"))
	f.Fuzz(func(t *testing.T, input []byte) {
		out, err := Parse(input)
		if err != nil {
			return
		}
		for _, o := range []Options{DefaultOptions(), {MaxIssues: 1}} {
			r := Build(out, o)
			res := core.Result{Text: Text(r), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			shown, typechecks, others := 0, 0, 0
			for _, p := range r.Issues {
				shown += max(p.Count, 1)
				if p.Linter == typecheck {
					typechecks += max(p.Count, 1)
				} else {
					others++
				}
			}
			if shown+r.IssuesOmitted != r.Total {
				t.Fatalf("%d issues shown and %d omitted, %d counted", shown, r.IssuesOmitted, r.Total)
			}
			if r.Total < len(out.Issues) {
				t.Fatalf("%d issues counted, %d in the input", r.Total, len(out.Issues))
			}
			if o.MaxIssues > 0 && others > o.MaxIssues {
				t.Fatalf("%d lines over the limit %d", others, o.MaxIssues)
			}
			for _, is := range out.Issues {
				if core.Clean(is.FromLinter) == typecheck && typechecks == 0 {
					t.Fatalf("typecheck issue cut:\n%s", res.Text)
				}
			}
			run := len(out.Report.Warnings) + len(out.Trailing)
			if core.Clean(out.Report.Error) != "" {
				run++
			}
			if r.RunErrors+r.RunWarnings != run {
				t.Fatalf("%d run diagnostics kept, %d in the input", r.RunErrors+r.RunWarnings, run)
			}
			if r.IssuesOmitted > 0 && !strings.Contains(res.Text, "(--max-issues 0)") {
				t.Fatalf("issues cut without a notice:\n%s", res.Text)
			}
			if (r.Total > 0 || r.RunErrors > 0) != strings.Contains(strings.Join(res.Conditions, ","), "fail") {
				t.Fatalf("fail condition %v with %d issues and %d run errors", res.Conditions, r.Total, r.RunErrors)
			}
			if _, err := core.Envelope("golangci", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

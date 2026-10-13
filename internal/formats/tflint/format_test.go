package tflint

import (
	"encoding/json"
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
		"empty run":     {`{"issues":[],"errors":[]}`, true},
		"leading space": {"\n {\"issues\":[{\"rule\":{}}],\"errors\":[]}", true},
		"no errors key": {`{"issues":[]}`, false},
		"other object":  {`{"format_version":"1.0","valid":true,"diagnostics":[]}`, false},
		"issues later":  {`{"results":{},"issues":[],"errors":[]}`, false},
		"array":         {`[{"issues":[],"errors":[]}]`, false},
		"tfjson stream": {`{"@level":"info","type":"version"}`, false},
		"plain text":    {"main.tf:1:1: Warning", false},
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
		"truncated":  `{"issues":[`,
		"no errors":  `{"issues":[]}`,
		"no issues":  `{"errors":[]}`,
		"wrong type": `{"issues":{},"errors":[]}`,
	} {
		if _, err := Squeeze([]byte(input), DefaultOptions()); err == nil {
			t.Errorf("%s: Squeeze() returned no error", name)
		}
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"issues":      {"fail", "fixable", "notice", "warning"},
		"hcl-error":   {"error", "fail"},
		"no-issues":   {},
		"modules":     {"error", "fail", "warning"},
		"fixed":       {"fail", "notice", "warning"},
		"notice-only": {"fail", "notice"},
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

func TestSeverities(t *testing.T) {
	t.Parallel()
	input := []byte(`{"issues":[` +
		`{"rule":{"name":"a","severity":"info"},"message":"n","range":{"filename":"a.tf","start":{"line":3,"column":1}},"callers":[]},` +
		`{"rule":{"name":"b","severity":"warning"},"message":"w","range":{"filename":"a.tf","start":{"line":2,"column":1}},"callers":[]},` +
		`{"rule":{"name":"c","severity":"critical"},"message":"u","range":{"filename":"b.tf","start":{"line":1,"column":1}},"callers":[]}],` +
		`"errors":[{"message":"plugin crashed","severity":"error"}]}`)
	r := report(t, input, DefaultOptions())
	if r.Errors != 1 || r.Warnings != 1 || r.Notices != 1 || r.ToolErrors != 1 {
		t.Errorf("counts = %+v", r)
	}
	var order []string
	for _, d := range r.Diagnostics {
		order = append(order, d.Severity+" "+d.Summary)
	}
	if diff := cmp.Diff([]string{"error plugin crashed", "error u", "warning w", "notice n"}, order); diff != "" {
		t.Errorf("order (-want +got):\n%s", diff)
	}
}

func TestLimitNamesTheOmittedRules(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "many"), Options{MaxIssues: 5})
	if err != nil {
		t.Fatal(err)
	}
	report := r.Data.(Report)
	if len(report.Diagnostics) != 5 || report.DiagnosticsOmitted != 116 {
		t.Fatalf("diagnostics = %d, omitted %d", len(report.Diagnostics), report.DiagnosticsOmitted)
	}
	want := []RuleCount{{Rule: "terraform_naming_convention", Count: 60}, {Rule: "terraform_unused_declarations", Count: 56}}
	if diff := cmp.Diff(want, report.OmittedRules); diff != "" {
		t.Errorf("omitted rules (-want +got):\n%s", diff)
	}
	if !strings.HasSuffix(r.Text, "... 116 more issues, terraform_naming_convention 60, terraform_unused_declarations 56 (--max-issues 0)\n") {
		t.Errorf("text:\n%s", r.Text)
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := []byte(`{"issues":[{"rule":{"name":"r\nerror: forged","severity":"warning"},"message":"m\u001b[31m\nerror: forged",` +
		`"range":{"filename":"a.tf\nerror: forged","start":{"line":1,"column":1}},"callers":[{"filename":"b.tf\nerror: forged","start":{"line":2,"column":1}}]}],` +
		`"errors":[{"summary":"s\nerror: forged","message":"x","severity":"warning"}]}`)
	for _, o := range []Options{DefaultOptions(), {NoDetail: true}} {
		r, err := Squeeze(input, o)
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
}

func TestDataPayload(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "no-issues"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	out, err := core.Envelope("tflint", 10, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Data struct {
			Diagnostics []any `json:"diagnostics"`
			Rules       []any `json:"rules"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &doc); err != nil || doc.Data.Diagnostics == nil || doc.Data.Rules == nil {
		t.Errorf("envelope = %s, err = %v", out, err)
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(`{"issues":[{"rule":{"severity":""},"range":{},"callers":[{}]}],"errors":[{"range":{}}]}`))
	f.Fuzz(func(t *testing.T, input []byte) {
		out, err := Parse(input)
		if err != nil {
			return
		}
		for _, o := range []Options{DefaultOptions(), {MaxIssues: 1, NoDetail: true}} {
			r := Build(out, o)
			res := core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			if got := r.Errors + r.Warnings + r.Notices; got != len(*out.Issues) {
				t.Fatalf("%d issues counted, %d in the input", got, len(*out.Issues))
			}
			if r.ToolErrors != len(*out.Errors) {
				t.Fatalf("%d tool errors counted, %d in the input", r.ToolErrors, len(*out.Errors))
			}
			if o.MaxIssues > 0 && len(r.Diagnostics) > o.MaxIssues {
				t.Fatalf("%d diagnostics over the limit %d", len(r.Diagnostics), o.MaxIssues)
			}
			if r.DiagnosticsOmitted > 0 && !strings.Contains(res.Text, "(--max-issues 0)") {
				t.Fatalf("issues cut without a notice:\n%s", res.Text)
			}
			if _, err := core.Envelope("tflint", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

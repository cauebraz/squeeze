package tflint

import (
	"encoding/json"
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
	data, err := os.ReadFile(filepath.Join(fixtureDir, name, "input.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func squeezeReport(t *testing.T, input []byte, o options) report {
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
		if _, err := squeeze([]byte(input), defaultOptions()); err == nil {
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
		r, err := squeeze(fixture(t, name), defaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, r.Conditions); diff != "" {
			t.Errorf("%s: conditions (-want +got):\n%s", name, diff)
		}
	}
}

func TestTrailingLines(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input string
		text  string
		conds []string
	}{
		"error after an empty report": {
			input: "{\"issues\":[],\"errors\":[]}\nError: Failed to load plugin\n",
			text:  "tflint: 0 issues; 1 tflint error, not every check ran\nerror: Failed to load plugin (tflint)\n",
			conds: []string{"error", "fail"},
		},
		"warning only": {
			input: "{\"issues\":[],\"errors\":[]}\nWARNING: flag is deprecated\nplain line\n",
			text:  "tflint: 0 issues\nwarning: flag is deprecated (tflint)\n",
			conds: []string{"warning"},
		},
		"plain lines only": {
			input: "{\"issues\":[],\"errors\":[]}\ndone\n",
			text:  "tflint: no issues\n",
			conds: []string{},
		},
	}
	for name, tc := range tests {
		r, err := squeeze([]byte(tc.input), defaultOptions())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if diff := cmp.Diff(tc.text, r.Text); diff != "" {
			t.Errorf("%s: text (-want +got):\n%s", name, diff)
		}
		if diff := cmp.Diff(tc.conds, r.Conditions); diff != "" {
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
	r := squeezeReport(t, input, defaultOptions())
	if r.Errors != 1 || r.Warnings != 1 || r.Notices != 1 || r.ToolErrors != 1 {
		t.Errorf("counts = %+v", r)
	}
	var order []string
	for _, d := range r.Problems {
		order = append(order, d.Severity+" "+d.Summary)
	}
	for _, is := range r.Issues {
		order = append(order, is.Severity+" "+is.Summary)
	}
	if diff := cmp.Diff([]string{"error plugin crashed", "error u", "warning w", "notice n"}, order); diff != "" {
		t.Errorf("order (-want +got):\n%s", diff)
	}
}

func TestLimitCountsLinesNotOccurrences(t *testing.T) {
	t.Parallel()
	dup := `{"rule":{"name":"r_dup","severity":"error"},"message":"d","range":{"filename":"a.tf","start":{"line":1,"column":1}},"callers":[]}`
	input := []byte(`{"issues":[` + dup + `,` + dup + `,` + dup + `,` +
		`{"rule":{"name":"r_err","severity":"error"},"message":"e","range":{"filename":"a.tf","start":{"line":2,"column":1}},"callers":[]},` +
		`{"rule":{"name":"r_warn","severity":"warning"},"message":"w","range":{"filename":"a.tf","start":{"line":3,"column":1}},"callers":[]}],"errors":[]}`)
	r, err := squeeze(input, options{MaxIssues: 2})
	if err != nil {
		t.Fatal(err)
	}
	rep := r.Data.(report)
	var listed []string
	for _, is := range rep.Issues {
		listed = append(listed, fmt.Sprintf("%s x%d", is.Rule, max(is.Count, 1)))
	}
	if diff := cmp.Diff([]string{"r_dup x3", "r_err x1"}, listed); diff != "" {
		t.Errorf("listed (-want +got):\n%s", diff)
	}
	if rep.IssuesOmitted != 1 || !strings.HasSuffix(r.Text, "... 1 more issue, r_warn 1 (--max-issues 0)\n") {
		t.Errorf("omitted %d, text:\n%s", rep.IssuesOmitted, r.Text)
	}
}

func TestFoldedIssuesCountInTheCut(t *testing.T) {
	t.Parallel()
	dup := `{"rule":{"name":"r_dup","severity":"warning"},"message":"d","range":{"filename":"a.tf","start":{"line":9,"column":1}},"callers":[]}`
	input := []byte(`{"issues":[` + dup + `,` + dup + `,` + dup + `,` +
		`{"rule":{"name":"r_err","severity":"error"},"message":"e1","range":{"filename":"a.tf","start":{"line":1,"column":1}},"callers":[],"fixed":true},` +
		`{"rule":{"name":"r_err","severity":"error"},"message":"e2","range":{"filename":"a.tf","start":{"line":2,"column":1}},"callers":[]}],` +
		`"errors":[{"message":"late","severity":"warning"}]}`)
	r, err := squeeze(input, options{MaxIssues: 2})
	if err != nil {
		t.Fatal(err)
	}
	rep := r.Data.(report)
	if len(rep.Issues) != 2 || rep.IssuesOmitted != 3 || len(rep.Problems) != 1 {
		t.Errorf("issues %d, omitted %d, problems %d", len(rep.Issues), rep.IssuesOmitted, len(rep.Problems))
	}
	if !strings.Contains(r.Text, "warning: late (tflint)\n") || !strings.HasSuffix(r.Text, "... 3 more issues, r_dup 3 (--max-issues 0)\n") {
		t.Errorf("text:\n%s", r.Text)
	}
	if !strings.Contains(r.Text, "(a.tf:1:1, r_err, fixed)") {
		t.Errorf("fixed marker missing:\n%s", r.Text)
	}
}

func TestCallerChainReadsRootToModule(t *testing.T) {
	t.Parallel()
	r, err := squeeze(fixture(t, "modules"), defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	rep := r.Data.(report)
	if rep.Issues[0].Detail != "evaluated in main.tf:20:19 -> modules/app/main.tf:4:19" || rep.Issues[1].Detail != "evaluated in modules/app/main.tf:4:19" {
		t.Errorf("details = %q, %q", rep.Issues[0].Detail, rep.Issues[1].Detail)
	}
}

func TestLimitNamesTheOmittedRules(t *testing.T) {
	t.Parallel()
	r, err := squeeze(fixture(t, "many"), options{MaxIssues: 5})
	if err != nil {
		t.Fatal(err)
	}
	rep := r.Data.(report)
	if len(rep.Issues) != 5 || rep.IssuesOmitted != 116 {
		t.Fatalf("issues = %d, omitted %d", len(rep.Issues), rep.IssuesOmitted)
	}
	want := []core.Count{{Name: "terraform_naming_convention", Count: 60}, {Name: "terraform_unused_declarations", Count: 56}}
	if diff := cmp.Diff(want, rep.OmittedByRule); diff != "" {
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
	for _, o := range []options{defaultOptions(), {NoDetail: true}} {
		r, err := squeeze(input, o)
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
	r, err := squeeze(fixture(t, "no-issues"), defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	out, err := core.Envelope("tflint", 10, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Data struct {
			Issues []any `json:"issues"`
			Rules  []any `json:"rules"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &doc); err != nil || doc.Data.Issues == nil || doc.Data.Rules == nil {
		t.Errorf("envelope = %s, err = %v", out, err)
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(`{"issues":[{"rule":{"severity":""},"range":{},"callers":[{}]}],"errors":[{"range":{}}]}`))
	f.Add([]byte("{\"issues\":[],\"errors\":[]}\nError: Failed to load plugin\nWarning: x\n"))
	f.Fuzz(func(t *testing.T, input []byte) {
		formattest.CheckStable(t, func() (core.Result, error) { return squeeze(input, defaultOptions()) })
		out, err := parse(input)
		if err != nil {
			return
		}
		for _, o := range []options{defaultOptions(), {MaxIssues: 1, NoDetail: true}} {
			r := build(out, o)
			res := core.Result{Text: text(r, o), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			if got := r.Errors + r.Warnings + r.Notices; got != len(*out.Issues) {
				t.Fatalf("%d issues counted, %d in the input", got, len(*out.Issues))
			}
			trailingErrors := 0
			for _, l := range out.Trailing {
				if l.Error {
					trailingErrors++
				}
			}
			if r.ToolErrors != len(*out.Errors)+trailingErrors {
				t.Fatalf("%d tool errors counted, %d in the input and %d after it", r.ToolErrors, len(*out.Errors), trailingErrors)
			}
			if len(out.Trailing) > 0 && !slices.Contains(res.Conditions, "error") && !slices.Contains(res.Conditions, "warning") {
				t.Fatalf("lines after the JSON set no condition: %v", res.Conditions)
			}
			shown := 0
			for _, is := range r.Issues {
				shown += max(is.Count, 1)
			}
			if shown+r.IssuesOmitted != len(*out.Issues) {
				t.Fatalf("%d issues shown and %d omitted, %d in the input", shown, r.IssuesOmitted, len(*out.Issues))
			}
			if o.MaxIssues > 0 && len(r.Issues) > o.MaxIssues {
				t.Fatalf("%d lines over the limit %d", len(r.Issues), o.MaxIssues)
			}
			if r.IssuesOmitted > 0 && !strings.Contains(res.Text, "(--max-issues 0)") {
				t.Fatalf("issues cut without a notice:\n%s", res.Text)
			}
			if _, err := core.Envelope("tflint", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

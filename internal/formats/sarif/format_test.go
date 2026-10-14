package sarif

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
	data, err := os.ReadFile(filepath.Join(fixtureDir, name, "input.sarif"))
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
		"compact":         {`{"version":"2.1.0","runs":[]}`, true},
		"pretty":          {"{\n  \"version\": \"2.1.0\",\n  \"runs\": []\n}", true},
		"schema first":    {`{"$schema":"https://json.schemastore.org/sarif-2.1.0.json","version":"2.1.0","runs":[]}`, true},
		"oasis schema":    {`{"$schema":"https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json","runs":[]}`, true},
		"no runs":         {`{"version":"2.1.0"}`, false},
		"other version":   {`{"version":"1.0.0","runs":[]}`, false},
		"array":           {`[{"version":"2.1.0","runs":[]}]`, false},
		"trivy json":      {`{"SchemaVersion":2,"ArtifactName":".","Results":[]}`, false},
		"tflint json":     {`{"issues":[],"errors":[]}`, false},
		"eslint json":     {`[{"filePath":"/a.js","messages":[],"errorCount":0}]`, false},
		"terraform state": {`{"format_version":"1.0","terraform_version":"1.16.4","values":{}}`, false},
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
		"truncated":     `{"version":"2.1.0","runs":[`,
		"no runs":       `{"version":"2.1.0"}`,
		"other version": `{"version":"2.0.0","runs":[]}`,
		"runs object":   `{"version":"2.1.0","runs":{}}`,
	} {
		if _, err := Squeeze([]byte(input), DefaultOptions()); err == nil {
			t.Errorf("%s: Squeeze() returned no error", name)
		}
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"trivy-fs":     {"error", "fail", "note", "warning"},
		"trivy-config": {"error", "fail", "note", "warning"},
		"tflint":       {"fail", "warning"},
		"checkov":      {"error", "fail"},
		"spec":         {"error", "fail", "note", "warning"},
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
	notes := report(t, []byte(`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"x"}},"results":[{"ruleId":"r","level":"note","message":{"text":"n"}}]}]}`), DefaultOptions())
	if got := conditions(notes); !cmp.Equal(got, []string{"note"}) {
		t.Errorf("notes only: conditions = %v, want [note]", got)
	}
}

func TestSpecSemantics(t *testing.T) {
	t.Parallel()
	r := report(t, fixture(t, "spec"), Options{})
	if r.Errors != 3 || r.Warnings != 2 || r.Notes != 1 || r.Suppressed != 1 || r.ToolErrors != 2 {
		t.Errorf("counts = %+v", r)
	}
	if diff := cmp.Diff([]core.Count{{Name: "pass", Count: 1}, {Name: "review", Count: 1}}, r.OtherKinds); diff != "" {
		t.Errorf("other kinds (-want +got):\n%s", diff)
	}
	var got []string
	for _, res := range r.Results {
		got = append(got, fmt.Sprintf("%s %s %s:%d:%d %s", res.Severity, res.Rule, res.File, res.Line, res.Column, res.Summary))
	}
	want := []string{
		"error EX001 /home/dev/shop/src/config files/db.js:4:7 Hard-coded credential in source",
		"error EX001 src/legacy.js:9:0 Hard-coded credential under review",
		"error EX002 src/order.js:0:0 Function is too long (240 lines)",
		"warning EX006 :0:0 no message",
		"warning EX002 src/cart.js:10:0 Function is too long (84 lines)",
		"note EX003 src/cart.js:12:3 Prefer template literals",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("results (-want +got):\n%s", diff)
	}
	var notes []string
	for _, d := range r.Notifications {
		notes = append(notes, d.Severity+" "+d.Summary)
	}
	if diff := cmp.Diff([]string{"error execution was not successful", "error could not read src/vendor/big.js: file too large", "warning rule EX009 is unknown and was ignored"}, notes); diff != "" {
		t.Errorf("notifications (-want +got):\n%s", diff)
	}
}

func TestSnippetsNeverReachTheOutput(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"spec", "checkov"} {
		r, err := Squeeze(fixture(t, name), Options{})
		if err != nil {
			t.Fatal(err)
		}
		env, err := core.Envelope("sarif", 10, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"hunter2", "cidr_blocks"} {
			if strings.Contains(r.Text, secret) || strings.Contains(string(env), secret) {
				t.Errorf("%s: snippet text %q reached the output", name, secret)
			}
		}
	}
}

func TestMultiLineMessageUsesTheRuleDescription(t *testing.T) {
	t.Parallel()
	r := report(t, fixture(t, "trivy-fs"), DefaultOptions())
	first := r.Results[0]
	if !strings.HasPrefix(first.Summary, "python-requests: ") || !strings.HasPrefix(first.Detail, "Package: requests\n") {
		t.Errorf("summary %q, detail %q", first.Summary, first.Detail)
	}
	one := report(t, []byte(`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"x","rules":[{"id":"r","shortDescription":{"text":"rule text"}}]}},"results":[{"ruleId":"r","message":{"text":"own text"}}]}]}`), DefaultOptions())
	if one.Results[0].Summary != "own text" {
		t.Errorf("one-line message replaced: %q", one.Results[0].Summary)
	}
}

func TestLimitNamesTheOmittedRules(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "trivy-config"), Options{MaxResults: 3})
	if err != nil {
		t.Fatal(err)
	}
	data := r.Data.(Report)
	sum := 0
	for _, c := range data.OmittedByRule {
		sum += c.Count
	}
	if len(data.Results) != 3 || data.ResultsOmitted != 8 || sum != 8 || !strings.Contains(r.Text, "... 8 more results, ") {
		t.Errorf("results %d, omitted %d, by rule %d, text:\n%s", len(data.Results), data.ResultsOmitted, sum, r.Text)
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := []byte(`{"version":"2.1.0","runs":[{"tool":{"driver":{"name":"x\nerror: forged","version":"1\u001b[31m"}},` +
		`"invocations":[{"toolExecutionNotifications":[{"level":"error","message":{"text":"n\u001b[2J"}}]}],` +
		`"results":[{"ruleId":"r\nerror: forged","message":{"text":"m\u001b[31m"},"locations":[{"physicalLocation":{"artifactLocation":{"uri":"a.tf\nerror: forged"}}}]}]}]}`)
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

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(`{"version":"2.1.0","runs":[{"results":[{"ruleIndex":-1,"kind":"","level":"x","locations":[{"physicalLocation":{"artifactLocation":{"uri":"%zz"},"region":{"startLine":-4}}}]}],"invocations":[{"executionSuccessful":false}]}]}`))
	f.Fuzz(func(t *testing.T, input []byte) {
		log, err := Parse(input)
		if err != nil {
			return
		}
		for _, o := range []Options{DefaultOptions(), {MaxResults: 1, NoDetail: true}} {
			r := Build(log, o)
			res := core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			shown := 0
			for _, x := range r.Results {
				shown += max(x.Count, 1)
			}
			if shown+r.ResultsOmitted != r.Errors+r.Warnings+r.Notes {
				t.Fatalf("%d results shown and %d omitted, %d counted", shown, r.ResultsOmitted, r.Errors+r.Warnings+r.Notes)
			}
			if o.MaxResults > 0 && len(r.Results) > o.MaxResults {
				t.Fatalf("%d lines over the limit %d", len(r.Results), o.MaxResults)
			}
			if r.ResultsOmitted > 0 && !strings.Contains(res.Text, "(--max-results 0)") {
				t.Fatalf("results cut without a notice:\n%s", res.Text)
			}
			if _, err := core.Envelope("sarif", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

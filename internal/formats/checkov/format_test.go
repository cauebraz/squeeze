package checkov

import (
	"os"
	"path/filepath"
	"slices"
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
		"report":       {`{"check_type":"terraform","results":{},"summary":{}}`, true},
		"list":         {`[{"check_type":"terraform","results":{}}]`, true},
		"summary only": {`{"passed":0,"failed":0,"resource_count":0,"checkov_version":"3.3.29"}`, true},
		"tflint":       {`{"issues":[],"errors":[]}`, false},
		"tfvalidate":   {`{"format_version":"1.0","valid":true,"diagnostics":[]}`, false},
		"list summary": {`[{"checkov_version":"3","resource_count":0}]`, false},
		"plain text":   {"Check: CKV_AWS_18 FAILED", false},
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
		"truncated":       `{"check_type":"terraform","results":`,
		"no results":      `{"check_type":"terraform","summary":{}}`,
		"list no summary": `[{"check_type":"terraform","results":{}}]`,
		"other object":    `{"issues":[],"errors":[]}`,
		"empty":           ` `,
	} {
		if _, err := Squeeze([]byte(input), DefaultOptions()); err == nil {
			t.Errorf("%s: Squeeze() returned no error", name)
		}
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"single":   {"fail", "skip"},
		"multi":    {"fail", "skip"},
		"quiet":    {"fail", "skip"},
		"broken":   {"error"},
		"empty":    {},
		"severity": {"critical", "fail", "high", "skip"},
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

func TestCountsComeFromTheSummary(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"single", "quiet", "compact"} {
		r := report(t, fixture(t, name), DefaultOptions())
		if r.Failed != 18 || r.Passed != 14 || r.Skipped != 1 || len(r.FailedChecks) != 18 {
			t.Errorf("%s: failed %d, passed %d, skipped %d, listed %d", name, r.Failed, r.Passed, r.Skipped, len(r.FailedChecks))
		}
	}
	multi := report(t, fixture(t, "multi"), DefaultOptions())
	var frameworks []string
	for _, f := range multi.Frameworks {
		frameworks = append(frameworks, f.CheckType)
	}
	if diff := cmp.Diff([]string{"terraform", "dockerfile"}, frameworks); diff != "" || multi.Failed != 22 {
		t.Errorf("multi: frameworks (-want +got):\n%s, failed %d", diff, multi.Failed)
	}
}

func TestGroupsAreOrderedByWorstSeverityThenLocation(t *testing.T) {
	t.Parallel()
	input := []byte(`{"check_type":"terraform","summary":{"failed":4,"checkov_version":"3"},"results":{"failed_checks":[` +
		`{"check_id":"A","check_name":"a","file_path":"/b.tf","file_line_range":[1,2],"resource":"r.low"},` +
		`{"check_id":"B","check_name":"b","file_path":"/a.tf","file_line_range":[9,9],"resource":"r.late","severity":"LOW"},` +
		`{"check_id":"C","check_name":"c","file_path":"/a.tf","file_line_range":[3,4],"resource":"r.early","severity":"HIGH"},` +
		`{"check_id":"D","check_name":"d","file_path":"/a.tf","file_line_range":[3,4],"resource":"r.early","severity":"CRITICAL"}],` +
		`"passed_checks":[],"skipped_checks":[],"parsing_errors":[]}}`)
	r, err := Squeeze(input, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	want := "checkov 3: 4 failed, 0 passed, 0 skipped; 0 resources\n" +
		"r.early (a.tf:3-4): 2 failed checks\n  CRITICAL D d\n  HIGH C c\n" +
		"r.late (a.tf:9): 1 failed check\n  LOW B b\n" +
		"r.low (b.tf:1-2): 1 failed check\n  A a\n"
	if diff := cmp.Diff(want, r.Text); diff != "" {
		t.Errorf("text (-want +got):\n%s", diff)
	}
}

func TestNullResultsAndCountsOnly(t *testing.T) {
	t.Parallel()
	list := []byte(`[{"check_type":"terraform","results":{"failed_checks":[],"passed_checks":[],"skipped_checks":[],"parsing_errors":[]},"summary":{"passed":1,"checkov_version":"3"}},` +
		`{"check_type":"dockerfile","results":null,"summary":{"passed":2,"checkov_version":"3"}}]`)
	if r := report(t, list, DefaultOptions()); r.Passed != 3 || len(r.Frameworks) != 2 {
		t.Errorf("report = %+v", r)
	}
	counts := []byte(`{"passed":3,"failed":2,"skipped":1,"parsing_errors":0,"resource_count":2,"checkov_version":"3.3.29"}`)
	r, err := Squeeze(counts, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(r.Text, "checkov 3.3.29: 2 failed, 3 passed, 1 skipped; 2 resources; counts only, the input holds no checks\n") {
		t.Errorf("text:\n%s", r.Text)
	}
}

func TestUnknownSeverityIsNamed(t *testing.T) {
	t.Parallel()
	input := []byte(`{"check_type":"t","summary":{"failed":2,"checkov_version":"3"},"results":{"failed_checks":[` +
		`{"check_id":"A","check_name":"a","file_path":"/a.tf","resource":"r","severity":"Bogus"},` +
		`{"check_id":"B","check_name":"b","file_path":"/a.tf","resource":"r","severity":"low"}],"passed_checks":[],"skipped_checks":[],"parsing_errors":[]}}`)
	r := report(t, input, DefaultOptions())
	if r.FailedChecks[0].Severity != "LOW" || r.FailedChecks[1].Severity != "UNKNOWN" || r.BySeverity["UNKNOWN"] != 1 {
		t.Errorf("severities = %+v", r.FailedChecks)
	}
}

func TestLimitNamesTheOmittedChecks(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "single"), Options{MaxChecks: 4})
	if err != nil {
		t.Fatal(err)
	}
	report := r.Data.(Report)
	if len(report.FailedChecks) != 4 || report.ChecksOmitted != 14 || len(report.OmittedChecks) == 0 {
		t.Fatalf("listed %d, omitted %d, ids %v", len(report.FailedChecks), report.ChecksOmitted, report.OmittedChecks)
	}
	if !strings.Contains(r.Text, "... 14 more failed checks, ") || !strings.Contains(r.Text, "(--max-checks 0)\n") {
		t.Errorf("text:\n%s", r.Text)
	}
}

func TestCodeAndEvaluationsNeverReachTheOutput(t *testing.T) {
	t.Parallel()
	input := []byte(`{"check_type":"secrets","summary":{"failed":1,"checkov_version":"3"},"results":{"failed_checks":[` +
		`{"check_id":"CKV_SECRET_6","check_name":"Base64 High Entropy String","file_path":"/app.env","file_line_range":[2,2],` +
		`"resource":"25910f981e85ca04baf359199dd0bd4a3ae738b6","code_block":[[2,"TOKEN=hunter2-hunter2-hunter2"]],` +
		`"evaluations":{"token":{"value":"hunter2-hunter2-hunter2"}},"entity_tags":{"owner":"hunter2-hunter2-hunter2"}}],` +
		`"passed_checks":[],"skipped_checks":[],"parsing_errors":[]}}`)
	for _, o := range []Options{DefaultOptions(), {All: true}} {
		r, err := Squeeze(input, o)
		if err != nil {
			t.Fatal(err)
		}
		env, err := core.Envelope("checkov", len(input), r, nil)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(r.Text, "hunter2") || strings.Contains(string(env), "hunter2") {
			t.Errorf("value from the code block reached the output:\n%s\n%s", r.Text, env)
		}
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := []byte(`{"check_type":"t\nX","summary":{"failed":1,"skipped":1,"checkov_version":"3\nX"},"results":{` +
		`"failed_checks":[{"check_id":"C\nX","check_name":"n\u001b[31m\nX","file_path":"/f\nX","resource":"r\nX","severity":"HIGH\nX","caller_file_path":"/c\nX"}],` +
		`"skipped_checks":[{"check_id":"S","check_name":"s","file_path":"/f","resource":"r","check_result":{"suppress_comment":"why\nX"}}],` +
		`"passed_checks":[],"parsing_errors":["p\nX"]}}`)
	for _, o := range []Options{DefaultOptions(), {All: true}} {
		r, err := Squeeze(input, o)
		if err != nil {
			t.Fatal(err)
		}
		formattest.CheckText(t, r.Text)
		for _, line := range strings.Split(r.Text, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "X") {
				t.Errorf("input forged a line: %q\n%s", line, r.Text)
			}
		}
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(`[{"check_type":"x","summary":{"failed":-3},"results":{"failed_checks":[{"file_line_range":[5]},{"severity":"high"}]}}]`))
	f.Add([]byte("{\"summary\":{\"checkov_version\":\"3\"}}\nError: x\nWarning: y\n"))
	f.Fuzz(func(t *testing.T, input []byte) {
		formattest.CheckStable(t, func() (core.Result, error) { return Squeeze(input, DefaultOptions()) })
		in, err := Parse(input)
		if err != nil {
			return
		}
		listed := 0
		for _, rep := range in.Reports {
			listed += len(rep.Results.FailedChecks)
		}
		for _, o := range []Options{DefaultOptions(), {MaxChecks: 1, All: true}} {
			r := Build(in, o)
			res := core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			if got := len(r.FailedChecks) + r.ChecksOmitted; got != listed {
				t.Fatalf("%d failed checks kept or omitted, %d in the input", got, listed)
			}
			for _, l := range in.Trailing {
				if !strings.Contains(res.Text, l.Text) {
					t.Fatalf("line after the JSON dropped: %q\n%s", l.Text, res.Text)
				}
				if l.Error && !slices.Contains(res.Conditions, "error") {
					t.Fatalf("error line %q sets no error condition", l.Text)
				}
			}
			if r.ChecksOmitted > 0 && !strings.Contains(res.Text, "(--max-checks 0)") {
				t.Fatalf("checks cut without a notice:\n%s", res.Text)
			}
			if _, err := core.Envelope("checkov", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

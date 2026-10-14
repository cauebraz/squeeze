package trivy

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
		"report":       {"{\n  \"SchemaVersion\": 2,\n  \"ArtifactName\": \"app\"}", true},
		"results only": {`{"SchemaVersion":2,"Results":[]}`, true},
		"no schema":    {`{"ArtifactName":"app","Results":[]}`, false},
		"checkov":      {`{"check_type":"terraform","results":{}}`, false},
		"tflint":       {`{"issues":[],"errors":[]}`, false},
		"array":        {`[{"SchemaVersion":2,"Results":[]}]`, false},
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
		"truncated": `{"SchemaVersion":2,"Results":[`,
		"no schema": `{"Results":[]}`,
		"schema 3":  `{"SchemaVersion":3,"Results":[]}`,
	} {
		if _, err := Squeeze([]byte(input), DefaultOptions()); err == nil {
			t.Errorf("%s: Squeeze() returned no error", name)
		}
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"fs":     {"critical", "fail", "fixable", "high", "secret", "vulnerability"},
		"config": {"fail", "high", "misconfiguration"},
		"clean":  {},
		"image":  {"critical", "fail", "fixable", "high", "misconfiguration", "vulnerability"},
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

func TestHiddenFindingsSetNoSeverityCondition(t *testing.T) {
	t.Parallel()
	misconf := func(id, severity, status string) Misconfiguration {
		return Misconfiguration{ID: id, Title: id, Severity: severity, Status: status}
	}
	tests := map[string]struct {
		results []Result
		o       Options
		want    []string
	}{
		"a critical exception": {
			results: []Result{{Target: "main.tf", Misconfigurations: []Misconfiguration{misconf("AVD-1", "CRITICAL", "EXCEPTION"), misconf("AVD-2", "LOW", "FAIL")}}},
			o:       DefaultOptions(),
			want:    []string{"fail", "misconfiguration"},
		},
		"a high exception beside a passed check": {
			results: []Result{{Target: "main.tf", Misconfigurations: []Misconfiguration{misconf("AVD-1", "HIGH", "EXCEPTION"), misconf("AVD-2", "HIGH", "PASS")}}},
			o:       DefaultOptions(),
			want:    []string{},
		},
		"a critical finding cut by --max-findings": {
			results: []Result{{Target: "main.tf", Misconfigurations: []Misconfiguration{misconf("AVD-1", "CRITICAL", "FAIL"), misconf("AVD-2", "LOW", "FAIL")}}},
			o:       Options{MaxFindings: 1},
			want:    []string{"critical", "fail", "high", "misconfiguration"},
		},
		"a critical finding listed with --all beside an exception": {
			results: []Result{{Target: "main.tf", Misconfigurations: []Misconfiguration{misconf("AVD-1", "CRITICAL", "EXCEPTION"), misconf("AVD-2", "HIGH", "FAIL")}}},
			o:       Options{All: true},
			want:    []string{"fail", "high", "misconfiguration"},
		},
	}
	for name, tt := range tests {
		raw := &RawReport{SchemaVersion: 2, ArtifactName: ".", ArtifactType: "filesystem", Results: tt.results}
		if diff := cmp.Diff(tt.want, conditions(Build(raw, tt.o))); diff != "" {
			t.Errorf("%s: conditions (-want +got):\n%s", name, diff)
		}
	}
}

func TestCounts(t *testing.T) {
	t.Parallel()
	r := report(t, fixture(t, "image"), DefaultOptions())
	got := []int{r.Vulnerabilities, r.Misconfigurations, r.Secrets, r.Licenses, r.SevereLicenses, r.Exceptions, r.Fixable, r.ChecksPassed}
	if diff := cmp.Diff([]int{5, 1, 0, 2, 1, 1, 3, 40}, got); diff != "" {
		t.Errorf("counts (-want +got):\n%s", diff)
	}
	if len(r.Hidden) != 2 {
		t.Errorf("hidden = %+v", r.Hidden)
	}
}

func TestResultsWithTheSameTargetStayApart(t *testing.T) {
	t.Parallel()
	input := []byte(`{"SchemaVersion":2,"ArtifactName":"app","Results":[` +
		`{"Target":"requirements.txt","Class":"lang-pkgs","Type":"pip","Vulnerabilities":[{"VulnerabilityID":"CVE-1","PkgName":"requests","InstalledVersion":"2.25.1","FixedVersion":"2.32.0","Severity":"HIGH"}]},` +
		`{"Target":"requirements.txt","Class":"secret","Secrets":[{"RuleID":"aws-access-key-id","Severity":"CRITICAL","Title":"AWS Access Key ID","StartLine":3,"EndLine":3}]}]}`)
	r, err := Squeeze(input, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	want := "trivy: app: 1 vulnerability, 1 secret; 1 CRITICAL, 1 HIGH; 1 fixable\n" +
		"requirements.txt (pip):\n  HIGH CVE-1 requests 2.25.1 -> 2.32.0\n" +
		"requirements.txt (secret):\n  CRITICAL aws-access-key-id line 3: AWS Access Key ID\n"
	if diff := cmp.Diff(want, r.Text); diff != "" {
		t.Errorf("text (-want +got):\n%s", diff)
	}
}

func TestLimitKeepsTheMostSevere(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "fs"), Options{MaxFindings: 5})
	if err != nil {
		t.Fatal(err)
	}
	report := r.Data.(Report)
	var kept []string
	for _, f := range report.Findings {
		kept = append(kept, f.Severity)
	}
	if diff := cmp.Diff([]string{"HIGH", "HIGH", "HIGH", "HIGH", "CRITICAL"}, kept); diff != "" {
		t.Errorf("kept severities in print order (-want +got):\n%s", diff)
	}
	if report.FindingsOmitted != 16 || report.OmittedBySeverity["MEDIUM"] != 10 || report.OmittedBySeverity["LOW"] != 1 {
		t.Errorf("omitted %d by severity %v", report.FindingsOmitted, report.OmittedBySeverity)
	}
	if !strings.HasSuffix(r.Text, "... 16 more findings, 5 HIGH, 10 MEDIUM, 1 LOW (--max-findings 0)\n") {
		t.Errorf("text:\n%s", r.Text)
	}
}

func TestSecretMatchNeverReachesTheOutput(t *testing.T) {
	t.Parallel()
	input := []byte(`{"SchemaVersion":2,"ArtifactName":"app","Results":[{"Target":"app.env","Class":"secret","Secrets":[` +
		`{"RuleID":"github-pat","Severity":"CRITICAL","Title":"GitHub Personal Access Token","StartLine":2,"EndLine":2,` +
		`"Match":"GITHUB_TOKEN=hunter2hunter2hunter2","Code":{"Lines":[{"Number":2,"Content":"GITHUB_TOKEN=hunter2hunter2hunter2"}]}}]}]}`)
	for _, o := range []Options{DefaultOptions(), {All: true}} {
		r, err := Squeeze(input, o)
		if err != nil {
			t.Fatal(err)
		}
		env, err := core.Envelope("trivy", len(input), r, nil)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(r.Text, "hunter2") || strings.Contains(string(env), "hunter2") {
			t.Errorf("secret reached the output:\n%s\n%s", r.Text, env)
		}
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := []byte(`{"SchemaVersion":2,"Trivy":{"Version":"0\nX"},"ArtifactName":"a\nX","ArtifactType":"t\nX","Results":[{"Target":"r\nX","Type":"y\nX",` +
		`"Vulnerabilities":[{"VulnerabilityID":"C\nX","PkgName":"p\nX","InstalledVersion":"1\nX","Status":"s\nX","Severity":"HIGH\nX","Title":"t\u001b[31m\nX"}],` +
		`"Misconfigurations":[{"ID":"M\nX","Title":"m\nX","Severity":"LOW","Status":"FAIL","CauseMetadata":{"Resource":"r\nX"}}],` +
		`"Licenses":[{"Severity":"LOW","Name":"L\nX","PkgName":"p\nX","Category":"c\nX"}]}]}`)
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
	f.Add([]byte(`{"SchemaVersion":2,"Results":[{"Misconfigurations":[{"Status":"EXCEPTION"},{"Status":"PASS"},{}],"Licenses":[{"Severity":"x"}],"Secrets":[{}]}]}`))
	f.Fuzz(func(t *testing.T, input []byte) {
		raw, err := Parse(input)
		if err != nil {
			return
		}
		for _, o := range []Options{DefaultOptions(), {MaxFindings: 1, All: true}} {
			r := Build(raw, o)
			res := core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			listed := r.Vulnerabilities + r.Misconfigurations + r.Secrets + r.SevereLicenses
			if got := len(r.Findings) + r.FindingsOmitted; got != listed {
				t.Fatalf("%d findings kept or omitted, %d counted", got, listed)
			}
			if r.FindingsOmitted > 0 && !strings.Contains(res.Text, "(--max-findings 0)") {
				t.Fatalf("findings cut without a notice:\n%s", res.Text)
			}
			if _, err := core.Envelope("trivy", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

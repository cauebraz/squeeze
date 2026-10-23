package npmaudit

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

func squeeze(t *testing.T, input string, o Options) core.Result {
	t.Helper()
	r, err := Squeeze([]byte(input), o)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func audit(vulns string) string {
	return `{"auditReportVersion":2,"vulnerabilities":{` + vulns + `},"metadata":{"vulnerabilities":{},"dependencies":{"prod":3,"dev":0,"optional":0,"peer":0,"peerOptional":0,"total":2}}}`
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
		"compact":              {`{"auditReportVersion":2,"vulnerabilities":{}}`, true},
		"pretty":               {"{\n  \"auditReportVersion\": 2,\n  \"vulnerabilities\": {}\n}", true},
		"no vulnerabilities":   {`{"auditReportVersion":2}`, false},
		"version later":        {`{"vulnerabilities":{},"auditReportVersion":2}`, false},
		"npm error":            {`{"error":{"code":"ENOLOCK","summary":"x","detail":"y"}}`, false},
		"npm 6 report":         {`{"actions":[],"advisories":{},"muted":[],"metadata":{"vulnerabilities":{}}}`, false},
		"pnpm audit":           {`{"advisories":{},"metadata":{"vulnerabilities":{"info":0},"dependencies":1}}`, false},
		"yarn audit":           {`{"type":"auditSummary","data":{"vulnerabilities":{"info":0}}}`, false},
		"trivy":                {`{"SchemaVersion":2,"ArtifactName":".","Results":[{"Vulnerabilities":[]}]}`, false},
		"eslint":               {`[{"filePath":"/a.js","messages":[],"errorCount":0}]`, false},
		"terraform outputs":    {`{"vulnerabilities":{"sensitive":false,"type":"string","value":"x"}}`, false},
		"npm audit text":       {"# npm audit report\n\nlodash  <=4.17.23\nSeverity: high\n", false},
		"short of the marker":  {`{"auditReport`, false},
		"other version marker": {`{"auditReportVersions":2,"vulnerabilities":{}}`, false},
	}
	for name, tt := range tests {
		if got := (Format{}).Detect([]byte(tt.input)); got != tt.want {
			t.Errorf("%s: Detect() = %v, want %v", name, got, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"truncated":          `{"auditReportVersion":`,
		"no version":         `{"vulnerabilities":{}}`,
		"no vulnerabilities": `{"auditReportVersion":2}`,
		"version 1":          `{"auditReportVersion":1,"vulnerabilities":{}}`,
		"via not a list":     `{"auditReportVersion":2,"vulnerabilities":{"a":{"via":{}}}}`,
		"fix not an object":  `{"auditReportVersion":2,"vulnerabilities":{"a":{"fixAvailable":"yes"}}}`,
		"array":              `[]`,
	}
	for name, input := range tests {
		if _, err := Squeeze([]byte(input), DefaultOptions()); err == nil {
			t.Errorf("%s: Squeeze() returned no error", name)
		}
	}
}

func TestNpmErrorIsReported(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input string
		want  string
	}{
		"no lockfile": {
			"{\n  \"error\": {\n    \"code\": \"ENOLOCK\",\n    \"summary\": \"This command requires an existing lockfile.\",\n    \"detail\": \"Try creating one first with: npm i --package-lock-only\\nOriginal error: loadVirtual requires existing shrinkwrap file\"\n  }\n}",
			"npm reported an error: ENOLOCK; This command requires an existing lockfile.; Try creating one first with: npm i --package-lock-only Original error: loadVirtual requires existing shrinkwrap file",
		},
		"registry unreachable": {
			`{"message":"request to http://127.0.0.1:9/-/npm/v1/security/audits/quick failed, reason: connect ECONNREFUSED 127.0.0.1:9","error":{"summary":"","detail":""}}`,
			"npm reported an error: request to http://127.0.0.1:9/-/npm/v1/security/audits/quick failed, reason: connect ECONNREFUSED 127.0.0.1:9",
		},
	}
	for name, tt := range tests {
		_, err := Squeeze([]byte(tt.input), DefaultOptions())
		if err == nil || err.Error() != tt.want {
			t.Errorf("%s: error = %v, want %q", name, err, tt.want)
		}
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"vulnerable": {"critical", "fail", "fixable", "high", "info", "low", "moderate"},
		"clean":      {},
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
	levels := map[string][]string{
		"info":     {"info"},
		"low":      {"fail", "info", "low"},
		"moderate": {"fail", "info", "low", "moderate"},
		"high":     {"fail", "high", "info", "low", "moderate"},
		"critical": {"critical", "fail", "high", "info", "low", "moderate"},
		"severe":   {"critical", "fail", "high", "info", "low", "moderate"},
		"":         {"critical", "fail", "high", "info", "low", "moderate"},
	}
	r := squeeze(t, audit(`"a":{"name":"a","severity":"severe (x)","via":[],"fixAvailable":false},"b":{"name":"b","severity":"low","via":[],"fixAvailable":false}`), Options{MaxVulnerabilities: 1})
	if want := "unknown a: no fix\n... 1 more vulnerability, low 1 (--max-vulnerabilities 0)\n"; !strings.HasSuffix(r.Text, want) {
		t.Errorf("unknown severity: text:\n%s\nwant suffix:\n%s", r.Text, want)
	}
	for sev, want := range levels {
		r := squeeze(t, audit(`"a":{"name":"a","severity":"`+sev+`","via":[],"fixAvailable":false}`), DefaultOptions())
		if diff := cmp.Diff(want, r.Conditions); diff != "" {
			t.Errorf("severity %q: conditions (-want +got):\n%s", sev, diff)
		}
	}
}

func TestFixAvailable(t *testing.T) {
	t.Parallel()
	r := squeeze(t, audit(
		`"a":{"name":"a","severity":"low","isDirect":true,"via":[],"range":"<1","fixAvailable":true},`+
			`"b":{"name":"b","severity":"low","isDirect":true,"via":[],"range":"<2","fixAvailable":{"name":"b","version":"2.0.0","isSemVerMajor":true}},`+
			`"c":{"name":"c","severity":"low","via":[],"range":"<3","fixAvailable":{"name":"x","version":"1.1.0","isSemVerMajor":false}},`+
			`"d":{"name":"d","severity":"low","via":[],"range":"*","effects":["x","y"],"fixAvailable":false},`+
			`"e":{"name":"e","severity":"low","via":[],"range":"*","effects":["x"],"fixAvailable":null}`), DefaultOptions())
	want := "npm audit: 5 vulnerabilities (5 low) in 2 dependencies; 2 direct; fix: 1 with npm audit fix, 2 with npm audit fix --force (1 breaking), 2 without one\n" +
		"low a <1 direct: fix with npm audit fix\n" +
		"low b <2 direct: fix b@2.0.0 (--force, breaking)\n" +
		"low c <3: fix x@1.1.0 (--force)\n" +
		"low d *: no fix; required by x, y\n" +
		"low e *: no fix; required by x\n"
	if r.Text != want {
		t.Errorf("text:\n%s\nwant:\n%s", r.Text, want)
	}
	if got := r.Data.(Report).Fixes; got != (Fixes{Auto: 1, Force: 1, Breaking: 1, None: 2}) {
		t.Errorf("fixes = %+v", got)
	}
}

func TestVulnerable(t *testing.T) {
	t.Parallel()
	r := squeeze(t, string(fixture(t, "vulnerable")), DefaultOptions())
	d := r.Data.(Report)
	wantSev := []core.Count{{Name: "critical", Count: 3}, {Name: "high", Count: 13}, {Name: "moderate", Count: 4}, {Name: "low", Count: 5}}
	if diff := cmp.Diff(wantSev, d.BySeverity); diff != "" {
		t.Errorf("by severity (-want +got):\n%s", diff)
	}
	if d.Total != 25 || d.Direct != 7 || d.Dependencies.Total != 301 || d.Dependencies.Dev != 191 || d.Fixes != (Fixes{Auto: 1, Force: 18, Breaking: 1, None: 5}) {
		t.Errorf("counts = total %d, direct %d, dependencies %+v, fixes %+v", d.Total, d.Direct, d.Dependencies, d.Fixes)
	}
	var lodash *Entry
	for i, e := range d.Vulnerabilities {
		if e.Name == "lodash" {
			lodash = &d.Vulnerabilities[i]
		}
	}
	if lodash == nil || len(lodash.Advisories) != 6 || lodash.Advisories[0].Severity != "high" || lodash.Advisories[5].Severity != "moderate" || !strings.HasPrefix(lodash.Advisories[0].ID, "GHSA-") {
		t.Errorf("lodash = %+v", lodash)
	}
	for i := 1; i < len(d.Vulnerabilities); i++ {
		if rank(d.Vulnerabilities[i-1].Severity) < rank(d.Vulnerabilities[i].Severity) {
			t.Errorf("%s listed before the more severe %s", d.Vulnerabilities[i-1].Name, d.Vulnerabilities[i].Name)
		}
	}
}

func TestAdvisoryID(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		a    Advisory
		want string
	}{
		"ghsa url":      {Advisory{Source: 1, URL: "https://github.com/advisories/GHSA-xvch-5gv4-984h"}, "GHSA-xvch-5gv4-984h"},
		"no url":        {Advisory{Source: 1099520}, "1099520"},
		"trailing path": {Advisory{Source: 7, URL: "/"}, "7"},
		"nothing":       {Advisory{}, "advisory"},
	}
	for name, tt := range tests {
		if got := advisoryID(&tt.a); got != tt.want {
			t.Errorf("%s: advisoryID() = %q, want %q", name, got, tt.want)
		}
	}
}

func TestAdvisoriesAndViaPackages(t *testing.T) {
	t.Parallel()
	r := squeeze(t, audit(
		`"m":{"name":"m","severity":"high","isDirect":true,"via":["p","q",null],"range":"1.x","fixAvailable":{"name":"m","version":"2.0.0","isSemVerMajor":false}},`+
			`"p":{"severity":"moderate","via":[{"source":1,"url":"https://github.com/advisories/GHSA-1","severity":"low"},{"source":2,"url":"https://github.com/advisories/GHSA-2","severity":"moderate"},{"source":3,"url":"https://github.com/advisories/GHSA-1","severity":"low"},"q"],"range":"*","effects":["m"],"fixAvailable":{"name":"m","version":"2.0.0","isSemVerMajor":false}}`), DefaultOptions())
	want := "npm audit: 2 vulnerabilities (1 high, 1 moderate) in 2 dependencies; 1 direct; fix: 2 with npm audit fix --force\n" +
		"high m 1.x direct: fix m@2.0.0 (--force); via p, q\n" +
		"moderate p *: fix m@2.0.0 (--force); GHSA-2, GHSA-1\n"
	if r.Text != want {
		t.Errorf("text:\n%s\nwant:\n%s", r.Text, want)
	}
}

func TestLimitCountsTheOmittedSeverities(t *testing.T) {
	t.Parallel()
	r := squeeze(t, string(fixture(t, "vulnerable")), Options{MaxVulnerabilities: 5})
	d := r.Data.(Report)
	if len(d.Vulnerabilities) != 5 || d.VulnerabilitiesOmitted != 20 {
		t.Fatalf("listed %d, omitted %d", len(d.Vulnerabilities), d.VulnerabilitiesOmitted)
	}
	want := []core.Count{{Name: "high", Count: 11}, {Name: "moderate", Count: 4}, {Name: "low", Count: 5}}
	if diff := cmp.Diff(want, d.OmittedBySeverity); diff != "" {
		t.Errorf("omitted by severity (-want +got):\n%s", diff)
	}
	if !strings.HasSuffix(r.Text, "... 20 more vulnerabilities, high 11, moderate 4, low 5 (--max-vulnerabilities 0)\n") {
		t.Errorf("text:\n%s", r.Text)
	}
	if diff := cmp.Diff([]string{"critical", "fail", "fixable", "high", "info", "low", "moderate"}, r.Conditions); diff != "" {
		t.Errorf("conditions after a cut (-want +got):\n%s", diff)
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	forged := `\nhigh forged: no fix\u001b[31m`
	r := squeeze(t, audit(`"a`+forged+`":{"name":"a`+forged+`","severity":"high`+forged+`","isDirect":false,"via":["v`+forged+`",{"source":1,"title":"t`+forged+`","url":"https://x/GHSA`+forged+`","severity":"low"}],"effects":["e`+forged+`"],"range":"<1`+forged+`","nodes":["n`+forged+`"],"fixAvailable":{"name":"f`+forged+`","version":"1`+forged+`"}}`), DefaultOptions())
	formattest.CheckText(t, r.Text)
	for _, line := range strings.Split(r.Text, "\n") {
		if strings.HasPrefix(line, "high forged") {
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
	out, err := core.Envelope("npmaudit", 10, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "null") {
		t.Errorf("envelope has a null collection: %s", out)
	}
	r = squeeze(t, audit(`"a":{"name":"a","severity":"low","via":[{"source":1}],"fixAvailable":false}`), DefaultOptions())
	out, err = core.Envelope("npmaudit", 10, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "null") {
		t.Errorf("envelope has a null collection: %s", out)
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(audit(`"a":{"severity":"bad","via":[null,"b",{"url":"/"}],"fixAvailable":{"name":"a"}},"b":{"name":"a","severity":"info","fixAvailable":true}`)))
	f.Fuzz(func(t *testing.T, input []byte) {
		formattest.CheckStable(t, func() (core.Result, error) { return Squeeze(input, DefaultOptions()) })
		a, err := Parse(input)
		if err != nil {
			return
		}
		for _, o := range []Options{DefaultOptions(), {MaxVulnerabilities: 1}} {
			r := Build(a, o)
			res := core.Result{Text: Text(r), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			formattest.CheckNotices(t, res.Text)
			if len(r.Vulnerabilities)+r.VulnerabilitiesOmitted != len(a.Vulnerabilities) || r.Total != len(a.Vulnerabilities) {
				t.Fatalf("%d listed and %d omitted, %d in the input", len(r.Vulnerabilities), r.VulnerabilitiesOmitted, len(a.Vulnerabilities))
			}
			if lines := strings.Count(res.Text, "\n"); lines != 1+len(r.Vulnerabilities)+min(r.VulnerabilitiesOmitted, 1) {
				t.Fatalf("%d lines for %d listed vulnerabilities:\n%s", lines, len(r.Vulnerabilities), res.Text)
			}
			sum := 0
			for _, c := range r.BySeverity {
				sum += c.Count
			}
			if sum != r.Total {
				t.Fatalf("severity counts sum to %d, total %d", sum, r.Total)
			}
			if (r.Total > 0) != slices.Contains(res.Conditions, "info") {
				t.Fatalf("total %d, conditions %v", r.Total, res.Conditions)
			}
			if r.VulnerabilitiesOmitted > 0 && !strings.Contains(res.Text, "(--max-vulnerabilities 0)") {
				t.Fatalf("vulnerabilities cut without a notice:\n%s", res.Text)
			}
			if _, err := core.Envelope("npmaudit", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

package tfjson

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
	"github.com/google/go-cmp/cmp"
)

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
		"version line":           {`{"@level":"info","@message":"Terraform 1.9.5","type":"version"}`, true},
		"after text lines":       {"Acquiring state lock\n\n" + `{"@level":"info","type":"version"}`, true},
		"too many text lines":    {"a\nb\nc\nd\ne\n" + `{"@level":"info","type":"version"}`, false},
		"no level":               {`{"type":"version"}`, false},
		"no type":                {`{"@level":"info"}`, false},
		"plan json":              {`{"format_version":"1.2","planned_values":{}}`, false},
		"validate json":          {`{"format_version":"1.0","valid":true,"diagnostics":[]}`, false},
		"pretty json with level": {"{\n  \"@level\": \"info\",\n  \"type\": \"x\"\n}", false},
		"empty":                  {"", false},
	}
	for name, tt := range tests {
		if got := (Format{}).Detect([]byte(tt.input)); got != tt.want {
			t.Errorf("%s: Detect() = %v, want %v", name, got, tt.want)
		}
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"plan":                {"change", "destroy", "replace"},
		"plan-diagnostics":    {"error", "fail", "warning"},
		"plan-drift-deferred": {"change", "deferred", "drift"},
		"import-move-forget":  {"change", "forget"},
		"apply":               {"change"},
		"apply-errored":       {"apply_errored", "change", "error", "fail"},
		"destroy":             {"change", "destroy"},
		"init":                {},
		"test-pass":           {},
		"test-fail":           {"error", "fail", "test_failed"},
		"policy-deny":         {"change", "fail", "policy_failed"},
		"unknown-types":       {"error", "fail", "warning"},
	}
	seen := map[string]bool{}
	for name, want := range tests {
		r, err := Squeeze(fixture(t, name), DefaultOptions())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if diff := cmp.Diff(want, r.Conditions); diff != "" {
			t.Errorf("%s: conditions (-want +got):\n%s", name, diff)
		}
		for _, c := range r.Conditions {
			seen[c] = true
		}
	}
	for _, c := range (Format{}).Conditions() {
		if !seen[c] {
			t.Errorf("declared condition %q has no fixture that produces it", c)
		}
	}
}

func TestHeader(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		r    Report
		want string
	}{
		"bare": {Report{Tool: "terraform"}, "terraform"},
		"init": {Report{Tool: "terraform", ToolVersion: "1.9.5", Operation: "init", Providers: 1}, "terraform 1.9.5 init: 1 provider installed"},
		"plan import actions": {Report{Tool: "terraform", Operation: "plan", Summary: &Summary{Operation: "plan", Import: 1, Add: 2, ActionInvocation: 2}},
			"terraform plan: 1 to import, 2 to add, 0 to change, 0 to destroy, 2 actions to invoke"},
		"apply import actions": {Report{Tool: "terraform", Operation: "apply", Summary: &Summary{Operation: "apply", Import: 1, Add: 1, ActionInvocation: 1, ActionFail: 1}},
			"terraform apply: 1 imported, 1 added, 0 changed, 0 destroyed, 1 action invoked, 1 action failed"},
		"destroy": {Report{Tool: "terraform", Operation: "destroy", Summary: &Summary{Operation: "destroy", Remove: 3}},
			"terraform destroy: 3 destroyed"},
		"unknown operation": {Report{Tool: "terraform", Operation: "refresh", Summary: &Summary{Operation: "refresh", Add: 1}},
			"terraform refresh: 1 add, 0 change, 0 destroy"},
		"interrupted apply": {Report{Tool: "terraform", Operation: "apply", Summary: &Summary{Operation: "plan", Add: 2}, Errors: 1},
			"terraform apply: no apply summary; plan was 2 to add, 0 to change, 0 to destroy; 1 error"},
		"tests": {Report{Tool: "terraform", Operation: "test", Tests: &Tests{Status: "fail", Passed: 1, Failed: 1}},
			"terraform test: fail, 1 passed, 1 failed, 0 errored, 0 skipped"},
		"diagnostics only": {Report{Tool: "terraform", ToolVersion: "1.9.5", Errors: 2, Warnings: 1}, "terraform 1.9.5: 2 errors, 1 warning"},
	}
	for name, tt := range tests {
		if got := header(tt.r); got != tt.want {
			t.Errorf("%s: header() =\n%q\nwant\n%q", name, got, tt.want)
		}
	}
}

func TestOutputsText(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	writeOutputs(&b, []Output{
		{Name: "a", Action: "create", Value: json.RawMessage(`"x"`)},
		{Name: "b", Action: "noop", Value: json.RawMessage(`"` + strings.Repeat("y", 60) + `"`)},
		{Name: "c", Sensitive: true, Value: json.RawMessage(`"secret"`)},
		{Name: "d", Value: json.RawMessage(`null`)},
	})
	if got := b.String(); got != "outputs: + a = \"x\", b, c (sensitive), d = null\n" {
		t.Errorf("writeOutputs() = %q", got)
	}
}

func TestOtherMessagesTruncated(t *testing.T) {
	t.Parallel()
	r := Report{Other: map[string]int{"log": 1}, otherMessages: []string{strings.Repeat("é", 150)}}
	var b strings.Builder
	writeOther(&b, r, true)
	line := strings.TrimPrefix(strings.TrimSuffix(b.String(), "\n"), "other:\n  ")
	if len(line) > maxMessageBytes || !strings.HasSuffix(line, "...") || !utf8Valid(line) {
		t.Errorf("truncated message = %d bytes %q", len(line), line[len(line)-8:])
	}
}

func utf8Valid(s string) bool {
	return strings.ToValidUTF8(s, "?") == s
}

func TestConfigureFlags(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"-max-resources", "x"}, {"-max-diagnostics", "-1"}} {
		fs := flag.NewFlagSet("tfjson", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		(Format{}).Configure(fs)
		if err := fs.Parse(args); err == nil {
			t.Errorf("Parse(%v) returned no error", args)
		}
	}
	var out bytes.Buffer
	fs := flag.NewFlagSet("tfjson", flag.ContinueOnError)
	fs.SetOutput(&out)
	(Format{}).Configure(fs)
	fs.PrintDefaults()
	for _, s := range []string{"(default 200)", "(default 20)", "-all", "-drift", "-no-detail"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("usage lacks %q", s)
		}
	}
}

func TestSqueezeError(t *testing.T) {
	t.Parallel()
	if _, err := Squeeze([]byte("plain text\n"), DefaultOptions()); err == nil {
		t.Error("Squeeze(plain text) returned no error")
	}
}

func TestSensitiveOutputsNeverPrinted(t *testing.T) {
	t.Parallel()
	input := stream(
		`{"@level":"info","type":"version","terraform":"1.9.5"}`,
		`{"@level":"info","@message":"Outputs: 1","type":"outputs","outputs":{"token":{"sensitive":true,"type":"string","value":"s3cr3t-value"}}}`,
	)
	r, err := Squeeze(input, Options{All: true})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := core.Envelope("tfjson", len(input), r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.Text, "s3cr3t-value") || bytes.Contains(envelope, []byte("s3cr3t-value")) {
		t.Errorf("sensitive output leaked:\n%s\n%s", r.Text, envelope)
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add(stream(`{"@level":"error","@message":"Error: x","type":"planned_change","change":{"resource":{"addr":"a"},"action":"create"}}`))
	f.Add(stream(`{"@level":"info","type":"apply_complete","hook":{"resource":{"addr":"a"},"action":{"addr":"x"},"elapsed_seconds":"x"}}`))
	f.Add(stream(`{"@level":"warn","type":"test_run","@testfile":"f","@testrun":"r","test_run":{"path":"f","run":"r","status":"fail"}}`))
	f.Fuzz(func(t *testing.T, input []byte) {
		lines, err := Parse(input)
		if err != nil {
			return
		}
		flagged := 0
		for _, l := range lines {
			if l.Level == "error" || l.Level == "warn" {
				flagged++
			}
		}
		for _, o := range []Options{DefaultOptions(), {All: true, ListDrift: true, MaxResources: 1, MaxDiagnostics: 1}} {
			r := Build(lines, o)
			if r.Errors+r.Warnings < flagged {
				t.Fatalf("%d error or warn lines but %d errors and %d warnings", flagged, r.Errors, r.Warnings)
			}
			res := core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r, lines)}
			formattest.CheckText(t, res.Text)
			if _, err := core.Envelope("tfjson", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := stream(
		`{"@level":"info","type":"version","terraform":"1.9\nerror: forged"}`,
		`{"@level":"info","type":"planned_change","change":{"resource":{"addr":"a.b\nerror: forged"},"action":"create\nerror: forged"}}`,
		`{"@level":"info","type":"outputs","outputs":{"o\nerror: forged":{"sensitive":false,"value":1}}}`,
		`{"@level":"info","@message":"x\nerror: forged","type":"future\nerror: forged"}`,
		`{"@level":"info","type":"change_summary","changes":{"add":1,"operation":"plan\nerror: forged\u001b"}}`,
		"text line \x00\x1b[31m",
	)
	for _, o := range []Options{DefaultOptions(), {All: true}} {
		r, err := Squeeze(input, o)
		if err != nil {
			t.Fatal(err)
		}
		formattest.CheckText(t, r.Text)
		for _, line := range strings.Split(r.Text, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "error:") {
				t.Errorf("input forged a line: %q\n%s", line, r.Text)
			}
		}
	}
}

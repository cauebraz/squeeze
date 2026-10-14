package tfjson

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func addresses(cs []Change) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, symbol(c.Action)+" "+c.Address)
	}
	return out
}

func TestBuildPlan(t *testing.T) {
	t.Parallel()
	r := build(t, "plan", DefaultOptions())
	want := []string{"- aws_instance.old", "-/+ aws_db_instance.main", "~ aws_instance.web", "+ aws_s3_bucket.logs", `+ aws_subnet.private["a"]`}
	if diff := cmp.Diff(want, addresses(r.Changes)); diff != "" {
		t.Errorf("changes (-want +got):\n%s", diff)
	}
	if r.Operation != "plan" || r.ToolVersion != "1.9.5" || r.Summary == nil || r.Summary.Add != 3 || r.Summary.Remove != 2 {
		t.Errorf("report = %+v, summary = %+v", r, r.Summary)
	}
	if diff := cmp.Diff(map[string]int{"apply_start": 1, "refresh": 1, "refresh_start": 1, "refresh_complete": 1}, r.Other); diff != "" {
		t.Errorf("other (-want +got):\n%s", diff)
	}
	all := build(t, "plan", Options{All: true})
	if len(all.Changes) != 6 || all.Changes[5].Action != "noop" {
		t.Errorf("--all changes = %v", addresses(all.Changes))
	}
	limited := build(t, "plan", Options{MaxResources: 2})
	if len(limited.Changes) != 2 || limited.Omitted != 3 {
		t.Errorf("limited = %d changes, %d omitted", len(limited.Changes), limited.Omitted)
	}
}

func TestBuildRepeatedPlannedChange(t *testing.T) {
	t.Parallel()
	replace := `{"@level":"info","type":"planned_change","change":{"resource":{"addr":"a.b"},"action":"replace","reason":"cannot_update"}}`
	deposed := `{"@level":"info","type":"planned_change","change":{"resource":{"addr":"a.b"},"action":"delete"}}`
	tests := map[string]struct {
		lines []string
		want  []string
	}{
		"a deposed object's delete beside the replace": {[]string{replace, deposed}, []string{"- a.b", "-/+ a.b"}},
		"the same order reversed":                      {[]string{deposed, replace}, []string{"- a.b", "-/+ a.b"}},
		"an identical line from a repeated stream":     {[]string{replace, deposed, replace}, []string{"- a.b", "-/+ a.b"}},
	}
	for name, tt := range tests {
		lines, err := Parse(stream(tt.lines...))
		if err != nil {
			t.Fatal(err)
		}
		r := Build(lines, DefaultOptions())
		if diff := cmp.Diff(tt.want, addresses(r.Changes)); diff != "" {
			t.Errorf("%s: changes (-want +got):\n%s", name, diff)
		}
	}
}

func TestBuildApply(t *testing.T) {
	t.Parallel()
	r := build(t, "apply", DefaultOptions())
	if r.Operation != "apply" || r.Applied.Count != 3 {
		t.Errorf("operation = %q, applied = %+v", r.Operation, r.Applied)
	}
	want := []Timing{{"aws_db_instance.main", 31}, {"aws_instance.web", 9.1}, {"aws_s3_bucket.logs", 2}}
	if diff := cmp.Diff(want, r.Applied.Slowest); diff != "" {
		t.Errorf("slowest (-want +got):\n%s", diff)
	}
	var outputs []string
	for _, o := range r.Outputs {
		outputs = append(outputs, o.Name+"="+string(o.Value))
	}
	if diff := cmp.Diff([]string{`bucket_name="acme-staging-logs"`, "db_password=", "replicas=2", "subnet_ids="}, outputs); diff != "" {
		t.Errorf("outputs (-want +got):\n%s", diff)
	}
	if !r.Outputs[1].Sensitive || r.Outputs[1].Value != nil {
		t.Errorf("sensitive output kept a value: %+v", r.Outputs[1])
	}
}

func TestBuildApplyErrored(t *testing.T) {
	t.Parallel()
	r := build(t, "apply-errored", DefaultOptions())
	if r.Operation != "apply" || r.Summary.Operation != "plan" {
		t.Errorf("operation = %q, summary operation = %q", r.Operation, r.Summary.Operation)
	}
	want := []Failure{{Type: "apply_errored", Address: "aws_instance.web", Message: "aws_instance.web: Creation errored after 4s"}}
	if diff := cmp.Diff(want, r.Failures); diff != "" {
		t.Errorf("failures (-want +got):\n%s", diff)
	}
	if r.Errors != 1 || r.Diagnostics[0].Address != "aws_instance.web" {
		t.Errorf("errors = %d, diagnostics = %+v", r.Errors, r.Diagnostics)
	}
}

func TestBuildPlanReadIsNotApply(t *testing.T) {
	t.Parallel()
	if r := build(t, "plan", DefaultOptions()); r.Operation != "plan" || r.Applied.Count != 0 {
		t.Errorf("a data source read turned a plan into %q with %d applied", r.Operation, r.Applied.Count)
	}
}

func TestBuildOperation(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"destroy": "destroy", "init": "init", "test-pass": "test", "test-fail": "test",
		"plan-diagnostics": "", "unknown-types": "plan", "noise-lines": "plan",
	}
	for name, want := range tests {
		if got := build(t, name, DefaultOptions()).Operation; got != want {
			t.Errorf("%s: operation = %q, want %q", name, got, want)
		}
	}
}

func TestBuildDiagnostics(t *testing.T) {
	t.Parallel()
	r := build(t, "plan-diagnostics", DefaultOptions())
	if r.Errors != 1 || r.Warnings != 2 || len(r.Diagnostics) != 2 || r.Diagnostics[1].Count != 2 {
		t.Errorf("diagnostics = %+v, errors %d, warnings %d", r.Diagnostics, r.Errors, r.Warnings)
	}
	limited := build(t, "plan-diagnostics", Options{MaxDiagnostics: 1})
	if len(limited.Diagnostics) != 1 || limited.DiagnosticsOmitted != 1 || limited.Diagnostics[0].Severity != "error" {
		t.Errorf("limited = %+v", limited.Diagnostics)
	}
}

func TestBuildNeverDropsErrors(t *testing.T) {
	t.Parallel()
	r := build(t, "unknown-types", DefaultOptions())
	var summaries []string
	for _, d := range r.Diagnostics {
		summaries = append(summaries, d.Severity+": "+d.Summary)
	}
	want := []string{"error: diagnostic with an unexpected shape", "error: future failure mode", "warning: deprecated thing in a log"}
	if diff := cmp.Diff(want, summaries); diff != "" {
		t.Errorf("diagnostics (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]int{"ephemeral": 1, "future_message": 1, "log": 1}, r.Other); diff != "" {
		t.Errorf("other (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"? aws_instance.web2"}, addresses(r.Changes)); diff != "" {
		t.Errorf("changes (-want +got):\n%s", diff)
	}
}

func TestBuildDriftDeferred(t *testing.T) {
	t.Parallel()
	r := build(t, "plan-drift-deferred", DefaultOptions())
	if diff := cmp.Diff([]string{"- aws_instance.bastion", "~ aws_security_group.web"}, addresses(r.Drift)); diff != "" {
		t.Errorf("drift (-want +got):\n%s", diff)
	}
	want := []Deferred{
		{Address: `aws_subnet.private["*"]`, Action: "create", Reason: "instance_count_unknown"},
		{Address: "kubernetes_namespace.app", Action: "create", Reason: "provider_config_unknown"},
	}
	if diff := cmp.Diff(want, r.Deferred); diff != "" {
		t.Errorf("deferred (-want +got):\n%s", diff)
	}
}

func TestBuildImportMoveForget(t *testing.T) {
	t.Parallel()
	r := build(t, "import-move-forget", DefaultOptions())
	want := []Change{
		{Address: "aws_s3_bucket.archive", Action: "remove"},
		{Address: "aws_s3_bucket.legacy", Action: "import", ImportID: "legacy-bucket"},
		{Address: "aws_iam_role.app", Action: "move", PreviousAddress: "aws_iam_role.legacy"},
	}
	if diff := cmp.Diff(want, r.Changes); diff != "" {
		t.Errorf("changes (-want +got):\n%s", diff)
	}
}

func TestBuildTests(t *testing.T) {
	t.Parallel()
	r := build(t, "test-fail", DefaultOptions())
	tests := r.Tests
	if tests == nil || tests.Status != "error" || tests.Passed != 1 || tests.Failed != 1 || tests.Errored != 1 {
		t.Fatalf("tests = %+v", tests)
	}
	var runs []string
	for _, run := range tests.Runs {
		runs = append(runs, run.File+" "+run.Run+" "+run.Status)
	}
	if diff := cmp.Diff([]string{"tests/bucket.tftest.hcl defaults pass", "tests/bucket.tftest.hcl versioning fail"}, runs); diff != "" {
		t.Errorf("runs (-want +got):\n%s", diff)
	}
	if len(tests.Runs[1].Diagnostics) != 1 || tests.Runs[1].Diagnostics[0].Summary != "Test assertion failed" {
		t.Errorf("run diagnostics = %+v", tests.Runs[1].Diagnostics)
	}
	if len(tests.Files) != 2 || tests.Files[1].Status != "error" || len(tests.Files[1].Diagnostics) != 1 {
		t.Errorf("files = %+v", tests.Files)
	}
	if len(r.Diagnostics) != 0 || r.Errors != 2 {
		t.Errorf("test diagnostics leaked to the top level: %+v, errors %d", r.Diagnostics, r.Errors)
	}
}

func TestBuildPolicy(t *testing.T) {
	t.Parallel()
	r := build(t, "policy-deny", DefaultOptions())
	want := []PolicyResult{{Policy: "policy.no_public_buckets", Target: "aws_s3_bucket.public", Result: "DenyResult", Enforcement: "mandatory"}}
	if diff := cmp.Diff(want, r.Policies); diff != "" {
		t.Errorf("policies (-want +got):\n%s", diff)
	}
	if r.Other["policy_result"] != 1 {
		t.Errorf("allowed policy result not counted: %v", r.Other)
	}
}

func TestBuildEmptyCollectionsAreArrays(t *testing.T) {
	t.Parallel()
	r := build(t, "init", DefaultOptions())
	for name, n := range map[string]int{
		"changes": len(r.Changes), "drift": len(r.Drift), "deferred": len(r.Deferred),
		"failures": len(r.Failures), "policies": len(r.Policies), "outputs": len(r.Outputs),
	} {
		if n != 0 {
			t.Errorf("%s has %d entries", name, n)
		}
	}
	if r.Changes == nil || r.Diagnostics == nil || r.Applied.Slowest == nil || r.Providers != 2 {
		t.Errorf("nil collections or wrong providers: %+v", r)
	}
}

func TestOtherMessagesKeepOrder(t *testing.T) {
	t.Parallel()
	r := build(t, "apply", DefaultOptions())
	if len(r.otherMessages) != 6 || !strings.HasPrefix(r.otherMessages[0], "aws_db_instance.main: Creating") {
		t.Errorf("other messages = %q", r.otherMessages)
	}
}

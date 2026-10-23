package tfjson

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/cauebraz/squeeze/internal/tfdiag"
)

func addresses(cs []change) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, symbol(c.Action)+" "+c.Address)
	}
	return out
}

func TestBuildPlan(t *testing.T) {
	t.Parallel()
	r := buildFixture(t, "plan", defaultOptions())
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
	all := buildFixture(t, "plan", options{All: true})
	if len(all.Changes) != 6 || all.Changes[5].Action != "noop" {
		t.Errorf("--all changes = %v", addresses(all.Changes))
	}
	limited := buildFixture(t, "plan", options{MaxResources: 2})
	if len(limited.Changes) != 2 || limited.Omitted != 3 {
		t.Errorf("limited = %d changes, %d omitted", len(limited.Changes), limited.Omitted)
	}
}

func TestBuildToolFromVersionLine(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		line          string
		tool, version string
	}{
		"terraform":  {`{"@level":"info","@message":"Terraform 1.9.5","terraform":"1.9.5","ui":"1.2","type":"version"}`, "terraform", "1.9.5"},
		"opentofu":   {`{"@level":"info","@message":"OpenTofu 1.13.1","@module":"tofu.ui","tofu":"1.13.1","ui":"1.2","type":"version"}`, "tofu", "1.13.1"},
		"no version": {`{"@level":"info","@message":"x","type":"log"}`, "terraform", ""},
	}
	for name, tt := range tests {
		lines, err := parse([]byte(tt.line + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		r := build(lines, defaultOptions())
		if r.Tool != tt.tool || r.ToolVersion != tt.version {
			t.Errorf("%s: tool = %q %q, want %q %q", name, r.Tool, r.ToolVersion, tt.tool, tt.version)
		}
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
		lines, err := parse(stream(tt.lines...))
		if err != nil {
			t.Fatal(err)
		}
		r := build(lines, defaultOptions())
		if diff := cmp.Diff(tt.want, addresses(r.Changes)); diff != "" {
			t.Errorf("%s: changes (-want +got):\n%s", name, diff)
		}
	}
}

func TestBuildApply(t *testing.T) {
	t.Parallel()
	r := buildFixture(t, "apply", defaultOptions())
	if r.Operation != "apply" || r.Applied.Count != 3 {
		t.Errorf("operation = %q, applied = %+v", r.Operation, r.Applied)
	}
	want := []timing{{"aws_db_instance.main", 31}, {"aws_instance.web", 9.1}, {"aws_s3_bucket.logs", 2}}
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
	r := buildFixture(t, "apply-errored", defaultOptions())
	if r.Operation != "apply" || r.Summary.Operation != "plan" {
		t.Errorf("operation = %q, summary operation = %q", r.Operation, r.Summary.Operation)
	}
	want := []failure{{Type: "apply_errored", Address: "aws_instance.web", Message: "aws_instance.web: Creation errored after 4s"}}
	if diff := cmp.Diff(want, r.Failures); diff != "" {
		t.Errorf("failures (-want +got):\n%s", diff)
	}
	if r.Errors != 1 || r.Diagnostics[0].Address != "aws_instance.web" {
		t.Errorf("errors = %d, diagnostics = %+v", r.Errors, r.Diagnostics)
	}
}

func TestBuildActionErroredAddress(t *testing.T) {
	t.Parallel()
	lines, err := parse(stream(
		`{"@level":"info","@message":"aws_instance.web (0): Action errored: action.local_command.notify - exit status 1","hook":{"action":{"addr":"action.local_command.notify","action_type":"local_command"},"error":"exit status 1","lifecycle":{"resource":{"addr":"aws_instance.web"},"trigger_index":0,"actions_index":0,"trigger_event":"AfterCreate","on_failure":"Fail"}},"type":"action_errored"}`,
		`{"@level":"info","@message":"Action errored: action.local_command.manual - exit status 2","hook":{"action":{"addr":"action.local_command.manual"},"error":"exit status 2","invoke":{}},"type":"action_errored"}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	r := build(lines, defaultOptions())
	want := []failure{
		{Type: "action_errored", Address: "action.local_command.notify", Message: "aws_instance.web (0): Action errored: action.local_command.notify - exit status 1"},
		{Type: "action_errored", Address: "action.local_command.manual", Message: "Action errored: action.local_command.manual - exit status 2"},
	}
	if diff := cmp.Diff(want, r.Failures); diff != "" {
		t.Errorf("failures (-want +got):\n%s", diff)
	}
}

func TestBuildPlanReadIsNotApply(t *testing.T) {
	t.Parallel()
	if r := buildFixture(t, "plan", defaultOptions()); r.Operation != "plan" || r.Applied.Count != 0 {
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
		if got := buildFixture(t, name, defaultOptions()).Operation; got != want {
			t.Errorf("%s: operation = %q, want %q", name, got, want)
		}
	}
}

func TestBuildDiagnostics(t *testing.T) {
	t.Parallel()
	r := buildFixture(t, "plan-diagnostics", defaultOptions())
	if r.Errors != 1 || r.Warnings != 2 || len(r.Diagnostics) != 2 || r.Diagnostics[1].Count != 2 {
		t.Errorf("diagnostics = %+v, errors %d, warnings %d", r.Diagnostics, r.Errors, r.Warnings)
	}
	limited := buildFixture(t, "plan-diagnostics", options{MaxDiagnostics: 1})
	if len(limited.Diagnostics) != 1 || limited.Diagnostics[0].Severity != "error" {
		t.Errorf("limited = %+v", limited.Diagnostics)
	}
	if limited.DiagnosticsOmitted != r.Diagnostics[1].Count {
		t.Errorf("omitted = %d, want %d: the cut entry stands for that many warnings", limited.DiagnosticsOmitted, r.Diagnostics[1].Count)
	}
}

func TestBuildNeverDropsErrors(t *testing.T) {
	t.Parallel()
	r := buildFixture(t, "unknown-types", defaultOptions())
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
	r := buildFixture(t, "plan-drift-deferred", defaultOptions())
	if diff := cmp.Diff([]string{"- aws_instance.bastion", "~ aws_security_group.web"}, addresses(r.Drift)); diff != "" {
		t.Errorf("drift (-want +got):\n%s", diff)
	}
	want := []deferred{
		{Address: `aws_subnet.private["*"]`, Action: "create", Reason: "instance_count_unknown"},
		{Address: "kubernetes_namespace.app", Action: "create", Reason: "provider_config_unknown"},
	}
	if diff := cmp.Diff(want, r.Deferred); diff != "" {
		t.Errorf("deferred (-want +got):\n%s", diff)
	}
}

func TestBuildImportMoveForget(t *testing.T) {
	t.Parallel()
	r := buildFixture(t, "import-move-forget", defaultOptions())
	want := []change{
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
	r := buildFixture(t, "test-fail", defaultOptions())
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

func TestBuildLeftInState(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		want   []leftInState
		errors int
		other  map[string]int
	}{
		"test-cleanup": {
			want: []leftInState{
				{File: "tests/bucket.tftest.hcl", Run: "setup", Reason: "cleanup_failed", Resources: []leftResource{
					{Address: "aws_iam_role.writer"}, {Address: "aws_s3_bucket.this"}, {Address: "aws_s3_bucket.this", DeposedKey: "4f2a91c0"},
				}},
				{File: "tests/keep.tftest.hcl", Run: "seed", Reason: "skip_cleanup", Resources: []leftResource{{Address: "aws_dynamodb_table.locks"}}},
			},
			errors: 2,
			other:  map[string]int{"test_abstract": 1},
		},
		"test-interrupt": {
			want: []leftInState{
				{File: "tests/vpc.tftest.hcl", Run: "peer", Reason: "interrupted", Resources: []leftResource{{Address: "aws_vpc.main"}, {Address: "aws_subnet.a"}}},
				{File: "tests/vpc.tftest.hcl", Run: "peer", Reason: "interrupted", State: "peer", Resources: []leftResource{{Address: "aws_vpc.peer"}}},
				{File: "tests/vpc.tftest.hcl", Run: "peer", Reason: "planned", Resources: []leftResource{{Address: "aws_vpc_peering_connection.this"}}},
			},
			errors: 1,
			other:  map[string]int{"log": 2, "test_abstract": 1},
		},
	}
	for name, tt := range tests {
		r := buildFixture(t, name, defaultOptions())
		if diff := cmp.Diff(tt.want, r.Tests.Left); diff != "" {
			t.Errorf("%s: left_in_state (-want +got):\n%s", name, diff)
		}
		if r.Errors != tt.errors {
			t.Errorf("%s: errors = %d, want %d", name, r.Errors, tt.errors)
		}
		if diff := cmp.Diff(tt.other, r.Other); diff != "" {
			t.Errorf("%s: other (-want +got):\n%s", name, diff)
		}
	}
}

func TestBuildPolicy(t *testing.T) {
	t.Parallel()
	r := buildFixture(t, "policy-deny", defaultOptions())
	want := []policyResult{{Policy: "policy.no_public_buckets", Target: "aws_s3_bucket.public", Result: "DenyResult", Outcome: "fail", Enforcement: "mandatory"}}
	if diff := cmp.Diff(want, r.Policies); diff != "" {
		t.Errorf("policies (-want +got):\n%s", diff)
	}
	if r.Other["policy_result"] != 1 {
		t.Errorf("allowed policy result not counted: %v", r.Other)
	}
}

func TestPolicyOutcome(t *testing.T) {
	t.Parallel()
	tests := map[string]outcome{
		"AllowResult": "pass", "DenyResult": "fail", "UnknownResult": "unknown",
		"InvalidResult": "error", "PolicyErrorResult": "error", "SetupErrorResult": "error",
		"pass": "pass", "n/a": "pass", "fail": "fail", "unknown": "unknown", "error": "error",
		"EvaluateResult(6)": "error",
	}
	for result, want := range tests {
		if got := policyOutcome(result); got != want {
			t.Errorf("policyOutcome(%q) = %q, want %q", result, got, want)
		}
	}
}

func TestBuildPolicyResults(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		policies         []policyResult
		errors, warnings int
		other            map[string]int
	}{
		"policy-unknown": {
			policies: []policyResult{{Policy: "resource_policy.no_public_acl", Target: "aws_s3_bucket.logs", Result: "UnknownResult", Outcome: "unknown", Enforcement: "mandatory"}},
			warnings: 1, other: map[string]int{},
		},
		"policy-error": {
			policies: []policyResult{{Policy: "resource_policy.no_public_acl", Target: "aws_s3_bucket.logs", Result: "PolicyErrorResult", Outcome: "error", Enforcement: "mandatory"}},
			errors:   1, other: map[string]int{},
		},
		"policy-setup-error": {
			policies: []policyResult{{Policy: "resource_policy.no_public_acl", Target: "aws_s3_bucket.logs", Result: "SetupErrorResult", Outcome: "error", Enforcement: "mandatory"}},
			errors:   1, other: map[string]int{},
		},
		"policy-invalid": {
			policies: []policyResult{{Policy: "resource_policy.no_public_acl", Target: "aws_s3_bucket.logs", Result: "InvalidResult", Outcome: "error", Enforcement: "advisory"}},
			other:    map[string]int{},
		},
		"policy-info": {
			policies: []policyResult{{Policy: "resource_policy.no_public_acl", Target: "aws_s3_bucket.logs", Result: "DenyResult", Outcome: "fail", Enforcement: "mandatory"}},
			errors:   1, other: map[string]int{"policy_info": 1, "policy_result": 1},
		},
		"query-policy": {
			policies: []policyResult{
				{Policy: "resource_policy.no_public_acl", Target: "list.aws_s3_bucket.all[1]", Result: "fail", Outcome: "fail", Enforcement: "mandatory"},
				{Policy: "resource_policy.no_public_acl", Target: "list.aws_s3_bucket.all[2]", Result: "unknown", Outcome: "unknown", Enforcement: "mandatory"},
				{Target: "list.aws_s3_bucket.all[3]", Result: "error", Outcome: "error"},
			},
			errors: 2, warnings: 1, other: map[string]int{},
		},
	}
	for name, tt := range tests {
		r := buildFixture(t, name, defaultOptions())
		if diff := cmp.Diff(tt.policies, r.Policies); diff != "" {
			t.Errorf("%s: policies (-want +got):\n%s", name, diff)
		}
		if r.Errors != tt.errors || r.Warnings != tt.warnings {
			t.Errorf("%s: errors, warnings = %d, %d, want %d, %d", name, r.Errors, r.Warnings, tt.errors, tt.warnings)
		}
		if diff := cmp.Diff(tt.other, r.Other); diff != "" {
			t.Errorf("%s: other (-want +got):\n%s", name, diff)
		}
	}
}

func TestPolicyDiagnosticKeepsDetail(t *testing.T) {
	t.Parallel()
	r := buildFixture(t, "policy-error", defaultOptions())
	want := []tfdiag.Diagnostic{{
		Severity: "error", Summary: "Unsupported attribute", Detail: `This object does not have an attribute named "acls".`,
		Address: "aws_s3_bucket.logs", File: "main.tf", Line: 1, Column: 1, Code: `resource "aws_s3_bucket" "logs" {`,
	}}
	if diff := cmp.Diff(want, r.Diagnostics); diff != "" {
		t.Errorf("diagnostics (-want +got):\n%s", diff)
	}
}

func TestQuerySummaryIdentityError(t *testing.T) {
	t.Parallel()
	lines, err := parse(stream(
		`{"@level":"info","@message":"Policy query summary","type":"policy_query_summary","list_block_address":"list.a.b","overall_result":"error","results":[{"target_address":"list.a.b[0]","result":"error","policies":[{"policy_metadata":{"policy_name":"p"},"diagnostics":null,"result":"pass"}]},{"target_address":"list.a.b[1]","result":"pass","policies":[{"policy_metadata":{"policy_name":"p"},"diagnostics":[{"severity":"warning","summary":"w"}],"result":"pass"}]}],"passed_policies":[]}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	r := build(lines, defaultOptions())
	want := []policyResult{{Target: "list.a.b[0]", Result: "error", Outcome: "error"}}
	if diff := cmp.Diff(want, r.Policies); diff != "" {
		t.Errorf("policies (-want +got):\n%s", diff)
	}
	if r.Warnings != 1 || r.Diagnostics[0].Address != "list.a.b[1]" {
		t.Errorf("warning from a passing policy dropped: %+v", r.Diagnostics)
	}
}

func TestBuildEmptyCollectionsAreArrays(t *testing.T) {
	t.Parallel()
	r := buildFixture(t, "init", defaultOptions())
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
	r := buildFixture(t, "apply", defaultOptions())
	if len(r.otherMessages) != 6 || !strings.HasPrefix(r.otherMessages[0], "aws_db_instance.main: Creating") {
		t.Errorf("other messages = %q", r.otherMessages)
	}
}

func TestBuildPlannedActions(t *testing.T) {
	t.Parallel()
	lifecycle := `{"@level":"info","type":"planned_action_invocation","invocation":{"action_addr":{"addr":"action.a.n"},"lifecycle_trigger":{"triggering_resource":{"addr":"aws_instance.web"},"triggering_event":"AfterCreate"}}}`
	lines, err := parse(stream(
		lifecycle,
		`{"@level":"info","type":"planned_action_invocation","invocation":{"action_addr":{"addr":"action.a.m"},"action_trigger":{}}}`,
		`{"@level":"info","type":"planned_action_invocation","invocation":{"action_addr":{"addr":"action.a.o"},"lifecycle_trigger":{"triggering_event":"BeforeDestroy"}}}`,
		`{"@level":"info","type":"planned_action_invocation","invocation":{"action_addr":{"addr":"action.a.p"}}}`,
		lifecycle,
		`{"@level":"info","type":"action_complete","hook":{"action":{"addr":"action.a.n"}}}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	r := build(lines, defaultOptions())
	want := []tfdiag.ActionInvocation{
		{Address: "action.a.n", Event: "AfterCreate", Resource: "aws_instance.web"},
		{Address: "action.a.m", Invoked: true},
		{Address: "action.a.o", Event: "BeforeDestroy"},
		{Address: "action.a.p"},
	}
	if diff := cmp.Diff(want, r.Actions); diff != "" {
		t.Errorf("actions (-want +got):\n%s", diff)
	}
	if r.ActionsCompleted != 1 || r.Operation != "plan" || len(r.Other) != 0 {
		t.Errorf("completed = %d, operation = %q, other = %v", r.ActionsCompleted, r.Operation, r.Other)
	}
	text := text(r, defaultOptions())
	wantText := "terraform plan\nactions: action.a.n (AfterCreate aws_instance.web), action.a.m (invoked), action.a.o (BeforeDestroy), action.a.p\nactions completed: 1\n"
	if diff := cmp.Diff(wantText, text); diff != "" {
		t.Errorf("text (-want +got):\n%s", diff)
	}
}

func TestBuildQueryIncomplete(t *testing.T) {
	t.Parallel()
	lines, err := parse(stream(
		`{"@level":"info","type":"list_resource_found","list_resource_found":{"address":"list.b.x","display_name":"","identity":{"id":"1"}}}`,
		`{"@level":"info","type":"list_start","list_start":{"address":"list.a.y","resource_type":"a"}}`,
		`{"@level":"info","type":"list_resource_found","list_resource_found":{"address":"list.a.y","display_name":"y-1"}}`,
		`{"@level":"info","type":"list_complete","list_complete":{"address":"list.a.y","resource_type":"a","total":1}}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	r := build(lines, defaultOptions())
	wantLists := []listBlock{
		{Address: "list.a.y", ResourceType: "a", Total: 1, Complete: true},
		{Address: "list.b.x", Total: 1},
	}
	if diff := cmp.Diff(wantLists, r.Lists); diff != "" {
		t.Errorf("lists (-want +got):\n%s", diff)
	}
	if r.Operation != "query" || len(r.Found) != 2 || r.Found[0].DisplayName != "y-1" || string(r.Found[1].Identity["id"]) != `"1"` {
		t.Errorf("operation = %q, found = %+v", r.Operation, r.Found)
	}
	text := text(r, defaultOptions())
	want := "terraform query: 2 found in 2 list blocks\nlist.a.y: 1 found\n  y-1\nlist.b.x: 1 found, incomplete\n  (no display name)\n"
	if diff := cmp.Diff(want, text); diff != "" {
		t.Errorf("text (-want +got):\n%s", diff)
	}
}

func TestBuildQueryKeepsInputAndStateOut(t *testing.T) {
	t.Parallel()
	r, err := squeeze(fixture(t, "query"), defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(r.Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"fixture-sensitive-input-value", "fixture-sensitive-state-value", "import {"} {
		if !strings.Contains(string(fixture(t, "query")), secret) {
			t.Fatalf("fixture lacks %q", secret)
		}
		if strings.Contains(r.Text, secret) || strings.Contains(string(data), secret) {
			t.Errorf("%q reached the output", secret)
		}
	}
}

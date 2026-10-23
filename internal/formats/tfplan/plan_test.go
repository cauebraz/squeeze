package tfplan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var defaults = DefaultOptions()

func addresses(cs []ResourceChange) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Action.Symbol()+" "+c.Address)
	}
	return out
}

func TestBuildOrderingAndTerraformCounts(t *testing.T) {
	t.Parallel()
	obj := map[string]any{}
	plan := &Plan{FormatVersion: "1.2", ResourceChanges: []PlanResource{
		{Address: "b.create", Change: PlanChange{Actions: []string{"create"}}},
		{Address: "a.create", Change: PlanChange{Actions: []string{"create"}}},
		{Address: "z.update", Change: PlanChange{Actions: []string{"update"}, Before: obj, After: obj}},
		{Address: "y.read", Change: PlanChange{Actions: []string{"read"}}},
		{Address: "x.import", Change: PlanChange{Actions: []string{"no-op"}, Importing: &PlanImporting{ID: "x"}}},
		{Address: "w.cbd", Change: PlanChange{Actions: []string{"create", "delete"}, Before: obj, After: obj}},
		{Address: "v.replace", Change: PlanChange{Actions: []string{"delete", "create"}, Before: obj, After: obj}},
		{Address: "u.delete", Change: PlanChange{Actions: []string{"delete"}}},
		{Address: "t.forget", Change: PlanChange{Actions: []string{"forget"}}},
		{Address: "s.fc", Change: PlanChange{Actions: []string{"forget", "create"}, Before: obj, After: obj}},
		{Address: "r.cf", Change: PlanChange{Actions: []string{"create", "forget"}, Before: obj, After: obj}},
		{Address: "u.delete", Deposed: "b", Change: PlanChange{Actions: []string{"delete"}}},
		{Address: "u.delete", Deposed: "a", Change: PlanChange{Actions: []string{"delete"}}},
		{Address: "q.noop", Change: PlanChange{Actions: []string{"no-op"}}},
	}}
	r, err := Build(plan, defaults)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"- u.delete", "- u.delete", "- u.delete", "-/+ v.replace", "+/- w.cbd", "./+ s.fc", "+/. r.cf",
		". t.forget", "~ z.update", "+ a.create", "+ b.create", "<= y.read", ">> x.import",
	}
	if diff := cmp.Diff(want, addresses(r.Changes)); diff != "" {
		t.Errorf("order (-want +got):\n%s", diff)
	}
	if got := []string{r.Changes[0].Deposed, r.Changes[1].Deposed, r.Changes[2].Deposed}; !cmp.Equal(got, []string{"", "a", "b"}) {
		t.Errorf("deposed order = %q", got)
	}
	wantSummary := Summary{Import: 1, Add: 6, Change: 1, Destroy: 5, Replace: 4, Forget: 3, Read: 1, Noop: 1, Complete: true}
	if diff := cmp.Diff(wantSummary, r.Summary); diff != "" {
		t.Errorf("summary (-want +got):\n%s", diff)
	}
}

func TestBuildTruncation(t *testing.T) {
	t.Parallel()
	before := map[string]any{}
	after := map[string]any{}
	for i := range 10 {
		before[fmt.Sprintf("a%d", i)] = i
		after[fmt.Sprintf("a%d", i)] = i + 1
	}
	plan := &Plan{FormatVersion: "1.2"}
	for i := range 5 {
		plan.ResourceChanges = append(plan.ResourceChanges, PlanResource{
			Address: fmt.Sprintf("r.n%d", i),
			Change:  PlanChange{Actions: []string{"update"}, Before: before, After: after},
		})
	}
	tests := map[string]struct {
		o                                     Options
		changes, omitted, attrs, attrsOmitted int
	}{
		"defaults":       {defaults, 5, 0, 8, 2},
		"limits":         {Options{MaxAttrs: 3, MaxResources: 2}, 2, 3, 3, 7},
		"zero means all": {Options{}, 5, 0, 10, 0},
		"exact limits":   {Options{MaxAttrs: 10, MaxResources: 5}, 5, 0, 10, 0},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, err := Build(plan, tt.o)
			if err != nil {
				t.Fatal(err)
			}
			got := []int{len(r.Changes), r.Omitted, len(r.Changes[0].Attrs), r.Changes[0].AttrsOmitted, r.Summary.Change}
			if diff := cmp.Diff([]int{tt.changes, tt.omitted, tt.attrs, tt.attrsOmitted, 5}, got); diff != "" {
				t.Errorf("changes, omitted, attrs, attrs omitted, summary (-want +got):\n%s", diff)
			}
		})
	}
}

func TestBuildNoopVisibility(t *testing.T) {
	t.Parallel()
	hidden := mustBuild(t, "noop-only", defaults)
	if len(hidden.Changes) != 0 || len(hidden.Outputs) != 0 || hidden.Summary.Noop != 2 || hidden.Summary.Changed() {
		t.Errorf("noop hidden: %+v", hidden)
	}
	shown := mustBuild(t, "noop-only", Options{IncludeNoop: true})
	if len(shown.Changes) != 2 || len(shown.Outputs) != 1 || shown.Summary.Outputs != 0 {
		t.Errorf("noop shown: changes = %d, outputs = %d, summary.Outputs = %d", len(shown.Changes), len(shown.Outputs), shown.Summary.Outputs)
	}
}

func TestBuildMovedNoopStaysVisible(t *testing.T) {
	t.Parallel()
	r := mustBuild(t, "moved", defaults)
	want := []string{". aws_s3_bucket.archive", "~ module.storage.aws_s3_bucket.this[0]", "= aws_iam_role.app"}
	if diff := cmp.Diff(want, addresses(r.Changes)); diff != "" {
		t.Errorf("changes (-want +got):\n%s", diff)
	}
	if r.Summary.Move != 2 || r.Summary.Forget != 1 || r.Summary.Change != 1 || r.Summary.Noop != 1 {
		t.Errorf("summary = %+v", r.Summary)
	}
}

func actionsFor(t *testing.T, a Action) []string {
	t.Helper()
	for key, candidate := range actionTable {
		if candidate == a {
			return strings.Split(key, ",")
		}
	}
	t.Fatalf("no actions list maps to %s", a)
	return nil
}

func TestBuildAttrsOnlyForUpdatesAndReplaces(t *testing.T) {
	t.Parallel()
	for _, a := range allActions {
		if a == Import {
			continue
		}
		plan := &Plan{FormatVersion: "1.2", ResourceChanges: []PlanResource{{
			Address: "r.x",
			Change: PlanChange{
				Actions: actionsFor(t, a),
				Before:  map[string]any{"a": "1"},
				After:   map[string]any{"a": "2"},
			},
		}}}
		r, err := Build(plan, Options{IncludeNoop: true})
		if err != nil {
			t.Fatalf("%s: %v", a, err)
		}
		if got, want := len(r.Changes[0].Attrs) > 0, hasAttrDiff(a); got != want {
			t.Errorf("%s: has attrs = %v, want %v", a, got, want)
		}
	}
}

func TestBuildDriftRelevance(t *testing.T) {
	t.Parallel()
	r := mustBuild(t, "drift", defaults)
	if diff := cmp.Diff([]string{"~ aws_s3_bucket.assets"}, addresses(r.Drift)); diff != "" {
		t.Errorf("relevant drift (-want +got):\n%s", diff)
	}
	if r.Summary.Drift != 1 || r.Summary.DriftIgnored != 2 || r.DriftOmitted != 2 {
		t.Errorf("drift counts = %d relevant, %d ignored, %d omitted", r.Summary.Drift, r.Summary.DriftIgnored, r.DriftOmitted)
	}
	all := mustBuild(t, "drift", Options{IncludeNoop: true})
	want := []string{"- aws_instance.bastion", "~ aws_s3_bucket.assets", "~ aws_s3_bucket.static"}
	if diff := cmp.Diff(want, addresses(all.Drift)); diff != "" {
		t.Errorf("all drift (-want +got):\n%s", diff)
	}
	if !all.Drift[0].Irrelevant || all.Drift[1].Irrelevant || !all.Drift[2].Irrelevant || all.DriftOmitted != 0 {
		t.Errorf("irrelevant flags = %v %v %v, omitted = %d", all.Drift[0].Irrelevant, all.Drift[1].Irrelevant, all.Drift[2].Irrelevant, all.DriftOmitted)
	}
}

func TestDriftRelevanceThroughListIndex(t *testing.T) {
	t.Parallel()
	rule := func(cidr string, port int) map[string]any {
		return map[string]any{"cidr": cidr, "port": json.Number(fmt.Sprint(port))}
	}
	tests := map[string]struct {
		before, after []any
		want          []string
	}{
		"swapped":          {[]any{rule("a", 22), rule("b", 443)}, []any{rule("b", 443), rule("a", 22)}, []string{"ingress"}},
		"moved and edited": {[]any{rule("a", 22), rule("b", 443)}, []any{rule("b", 443), rule("a", 80)}, []string{"ingress[0->1].port"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			plan := &Plan{FormatVersion: "1.2",
				ResourceDrift: []PlanResource{{Address: "r.sg", Change: PlanChange{
					Actions: []string{"update"},
					Before:  map[string]any{"ingress": tt.before},
					After:   map[string]any{"ingress": tt.after},
				}}},
				RelevantAttributes: []PlanRelevantAttr{{Resource: "r.sg", Attribute: []any{"ingress", json.Number("0"), "cidr"}}},
			}
			r, err := Build(plan, defaults)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Drift) != 1 || r.Summary.Drift != 1 {
				t.Fatalf("drift = %+v, summary = %+v", r.Drift, r.Summary)
			}
			var paths []string
			for _, a := range r.Drift[0].Attrs {
				paths = append(paths, a.Path)
			}
			if diff := cmp.Diff(tt.want, paths); diff != "" {
				t.Errorf("attrs (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDriftRelevanceRules(t *testing.T) {
	t.Parallel()
	upd := func(addr string) PlanResource {
		return PlanResource{Address: addr, Change: PlanChange{
			Actions: []string{"update"},
			Before:  map[string]any{"tags": map[string]any{"a": "1"}, "acl": "private"},
			After:   map[string]any{"tags": map[string]any{"a": "2"}, "acl": "public"},
		}}
	}
	del := PlanResource{Address: "r.gone", Change: PlanChange{Actions: []string{"delete"}, Before: map[string]any{"id": "x"}}}
	tests := map[string]struct {
		relevant  []PlanRelevantAttr
		wantDrift []string
		wantAttrs []string
	}{
		"no relevant attributes hides all drift": {nil, []string{}, nil},
		"resource path keeps matching attrs": {
			[]PlanRelevantAttr{{Resource: "r.a", Attribute: []any{"tags"}}},
			[]string{"~ r.a"}, []string{"tags.a"},
		},
		"global path applies to every resource": {
			[]PlanRelevantAttr{{Attribute: []any{"acl"}}},
			[]string{"- r.gone", "~ r.a", "~ r.b"}, []string{"acl"},
		},
		"path without a change hides the resource": {
			[]PlanRelevantAttr{{Resource: "r.a", Attribute: []any{"other"}}},
			[]string{}, nil,
		},
		"delete is relevant when its address is referenced": {
			[]PlanRelevantAttr{{Resource: "r.gone", Attribute: []any{"id"}}},
			[]string{"- r.gone"}, nil,
		},
		"invalid path is ignored": {
			[]PlanRelevantAttr{{Resource: "r.a", Attribute: []any{true}}},
			[]string{}, nil,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			plan := &Plan{FormatVersion: "1.2", ResourceDrift: []PlanResource{upd("r.a"), upd("r.b"), del}, RelevantAttributes: tt.relevant}
			r, err := Build(plan, defaults)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.wantDrift, addresses(r.Drift), cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("drift (-want +got):\n%s", diff)
			}
			for _, c := range r.Drift {
				if c.Action != Update {
					continue
				}
				var paths []string
				for _, a := range c.Attrs {
					paths = append(paths, a.Path)
				}
				if diff := cmp.Diff(tt.wantAttrs, paths); diff != "" {
					t.Errorf("%s attrs (-want +got):\n%s", c.Address, diff)
				}
			}
			if r.Summary.Drift+r.Summary.DriftIgnored != 3 {
				t.Errorf("drift counts %d + %d != 3", r.Summary.Drift, r.Summary.DriftIgnored)
			}
		})
	}
}

func TestBuildDeferred(t *testing.T) {
	t.Parallel()
	r := mustBuild(t, "deferred", defaults)
	want := []DeferredChange{
		{Address: "aws_subnet.private[\"*\"]", Action: Create, Reason: "instance_count_unknown"},
		{Address: "kubernetes_namespace.app", Action: Create, Reason: "provider_config_unknown"},
	}
	if diff := cmp.Diff(want, r.Deferred); diff != "" {
		t.Errorf("deferred (-want +got):\n%s", diff)
	}
	if r.Summary.Deferred != 2 || r.Summary.Complete {
		t.Errorf("summary = %+v", r.Summary)
	}
	moved := &Plan{FormatVersion: "1.2", DeferredChanges: []PlanDeferred{{Reason: "deferred_prereq", ResourceChange: PlanResource{
		Address: "b", PreviousAddress: "a", Change: PlanChange{Actions: []string{"no-op"}},
	}}}}
	got, err := Build(moved, defaults)
	if err != nil || len(got.Deferred) != 1 {
		t.Errorf("moved no-op deferred = %+v, %v", got.Deferred, err)
	}
}

func TestBuildActions(t *testing.T) {
	t.Parallel()
	r := mustBuild(t, "actions", defaults)
	want := []ActionInvocation{
		{Address: "action.aws_lambda_invoke.notify", Event: "AfterCreate", Resource: "aws_instance.web"},
		{Address: "action.aws_lambda_invoke.audit", Invoked: true},
	}
	if diff := cmp.Diff(want, r.Actions); diff != "" {
		t.Errorf("actions (-want +got):\n%s", diff)
	}
	if r.Summary.Actions != 2 {
		t.Errorf("summary.Actions = %d", r.Summary.Actions)
	}
}

func TestBuildDeferredActions(t *testing.T) {
	t.Parallel()
	r := mustBuild(t, "deferred-actions", defaults)
	want := []DeferredAction{
		{ActionInvocation{Address: "action.aws_lambda_invoke.notify", Event: "AfterCreate", Resource: "kubernetes_namespace.app"}, "deferred_prereq"},
		{ActionInvocation{Address: "action.aws_lambda_invoke.audit", Invoked: true}, "provider_config_unknown"},
	}
	if diff := cmp.Diff(want, r.DeferredActions); diff != "" {
		t.Errorf("deferred actions (-want +got):\n%s", diff)
	}
	if r.Summary.DeferredActions != 2 || r.Summary.Deferred != 1 || r.Summary.Actions != 1 {
		t.Errorf("summary = %+v", r.Summary)
	}
	only := &Plan{FormatVersion: "1.2", DeferredActions: []PlanDeferredAction{{Reason: "absent_prereq", ActionInvocation: PlanAction{Address: "action.a.b"}}}}
	got, err := Build(only, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if c := conditions(got); strings.Join(c, ",") != "deferred" {
		t.Errorf("conditions with only a deferred action = %v, want [deferred]", c)
	}
}

func TestBuildGeneratedConfig(t *testing.T) {
	t.Parallel()
	r := mustBuild(t, "generated-config", defaults)
	want := []ResourceChange{
		{Address: "aws_db_instance.legacy", Action: Import, Importing: true, GeneratedLines: 9},
		{Address: "aws_s3_bucket.legacy", Action: Import, Importing: true, GeneratedLines: 8},
	}
	if diff := cmp.Diff(want, r.Changes, cmpopts.IgnoreFields(ResourceChange{}, "GeneratedConfig")); diff != "" {
		t.Errorf("changes (-want +got):\n%s", diff)
	}
	if !strings.Contains(r.Changes[0].GeneratedConfig, "password            = null # sensitive\n") {
		t.Errorf("generated config = %q", r.Changes[0].GeneratedConfig)
	}
	hidden := mustBuild(t, "generated-config", Options{NoValues: true})
	if hidden.Changes[0].GeneratedConfig != "" || hidden.Changes[0].GeneratedLines != 9 {
		t.Errorf("--no-values change = %+v", hidden.Changes[0])
	}
}

func TestGeneratedConfigCleaning(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		in   string
		want string
	}{
		"empty":             {"", ""},
		"only newlines":     {"\n\n", ""},
		"trailing newline":  {"resource \"a\" \"b\" {\n}\n", "resource \"a\" \"b\" {\n}"},
		"crlf":              {"a\r\nb", "a\nb"},
		"escapes and tabs":  {"\x1b[31ma\x1b[0m\n\tb = 1\r", "a\n    b = 1"},
		"trailing spaces":   {"a   \nb", "a\nb"},
		"inner blank lines": {"a\n\nb", "a\n\nb"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := generatedConfig(tt.in); got != tt.want {
				t.Errorf("generatedConfig(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestBuildForgetReplace(t *testing.T) {
	t.Parallel()
	r := mustBuild(t, "forget-replace", defaults)
	if diff := cmp.Diff([]string{"./+ aws_instance.keep", "+/. aws_instance.keep_cbd"}, addresses(r.Changes)); diff != "" {
		t.Errorf("changes (-want +got):\n%s", diff)
	}
	if r.Summary.Add != 2 || r.Summary.Destroy != 0 || r.Summary.Replace != 2 || r.Summary.Forget != 2 {
		t.Errorf("summary = %+v", r.Summary)
	}
}

func TestBuildOutputs(t *testing.T) {
	t.Parallel()
	r := mustBuild(t, "outputs", defaults)
	want := []OutputChange{
		{Name: "bucket_name", Action: Delete},
		{Name: "db_endpoint", Action: Create, Unknown: true},
		{Name: "db_password", Action: Update, Sensitive: true},
	}
	if diff := cmp.Diff(want, r.Outputs); diff != "" || r.Summary.Outputs != 3 {
		t.Errorf("outputs (-want +got):\n%s, summary.Outputs = %d", diff, r.Summary.Outputs)
	}
}

func TestBuildImport(t *testing.T) {
	t.Parallel()
	r := mustBuild(t, "import", defaults)
	if diff := cmp.Diff([]string{"~ aws_iam_role.ci", ">> aws_s3_bucket.legacy"}, addresses(r.Changes)); diff != "" {
		t.Errorf("changes (-want +got):\n%s", diff)
	}
	if !r.Changes[0].Importing || r.Summary.Import != 2 || r.Summary.Change != 1 || r.Summary.Noop != 0 {
		t.Errorf("summary = %+v", r.Summary)
	}
	if a := r.Changes[0].Attrs[0]; a.Kind != Added || a.Path != "tags.ManagedBy" {
		t.Errorf("first attr = %+v, want added tags.ManagedBy", a)
	}
}

func TestBuildChecks(t *testing.T) {
	t.Parallel()
	check := func(kind, addr, status string, instances ...PlanCheckInstance) PlanCheck {
		return PlanCheck{Address: PlanCheckAddress{Kind: kind, ToDisplay: addr}, Status: status, Instances: instances}
	}
	instance := func(addr, status string, messages ...string) PlanCheckInstance {
		in := PlanCheckInstance{Address: PlanCheckAddress{ToDisplay: addr}, Status: status}
		for _, m := range messages {
			in.Problems = append(in.Problems, PlanCheckProblem{Message: m})
		}
		return in
	}
	tests := []struct {
		name       string
		checks     []PlanCheck
		errored    bool
		want       []CheckResult
		conditions []string
	}{
		{
			name:       "pass and unknown are not listed",
			checks:     []PlanCheck{check("check", "check.a", "pass", instance("check.a", "pass")), check("var", "var.b", "unknown")},
			want:       []CheckResult{},
			conditions: []string{},
		},
		{
			name:       "a failed check block is a warning",
			checks:     []PlanCheck{check("check", "check.a", "fail", instance("check.a", "fail", "down"))},
			want:       []CheckResult{{Address: "check.a", Kind: "check", Status: "fail", Problems: []string{"down"}}},
			conditions: []string{"check", "warning"},
		},
		{
			name:       "a failed resource condition in a plan that did not error is a warning",
			checks:     []PlanCheck{check("resource", "aws_instance.web", "fail", instance("aws_instance.web[0]", "pass"), instance("aws_instance.web[1]", "fail", "bad ami", "no tags"))},
			want:       []CheckResult{{Address: "aws_instance.web[1]", Kind: "resource", Status: "fail", Problems: []string{"bad ami", "no tags"}}},
			conditions: []string{"check", "warning"},
		},
		{
			name:       "a failed resource condition in an errored plan is the error",
			checks:     []PlanCheck{check("resource", "aws_instance.web", "fail", instance("aws_instance.web", "fail", "bad ami"))},
			errored:    true,
			want:       []CheckResult{{Address: "aws_instance.web", Kind: "resource", Status: "fail", Problems: []string{"bad ami"}}},
			conditions: []string{"check", "error", "fail"},
		},
		{
			name:       "a failed check block in an errored plan is still a warning",
			checks:     []PlanCheck{check("check", "check.a", "fail", instance("check.a", "fail", "down"))},
			errored:    true,
			want:       []CheckResult{{Address: "check.a", Kind: "check", Status: "fail", Problems: []string{"down"}}},
			conditions: []string{"check", "error", "fail", "warning"},
		},
		{
			name:       "an object without instances stands in for them",
			checks:     []PlanCheck{check("output_value", "output.x", "error")},
			want:       []CheckResult{{Address: "output.x", Kind: "output_value", Status: "error"}},
			conditions: []string{"check", "warning"},
		},
		{
			name:       "an unexpected status is listed",
			checks:     []PlanCheck{check("var", "var.v", "skipped", instance("", "skipped", ""))},
			want:       []CheckResult{{Address: "var.v", Kind: "var", Status: "skipped"}},
			conditions: []string{"check", "warning"},
		},
		{
			name:       "messages cannot forge a line",
			checks:     []PlanCheck{check("check", "check.a", "fail", instance("check.a", "fail", "down\n\x1b[31mfail check.b: forged"))},
			want:       []CheckResult{{Address: "check.a", Kind: "check", Status: "fail", Problems: []string{"down fail check.b: forged"}}},
			conditions: []string{"check", "warning"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, err := Build(&Plan{Checks: tt.checks, Errored: tt.errored}, defaults)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, r.Checks); diff != "" {
				t.Errorf("checks (-want +got):\n%s", diff)
			}
			if r.Summary.ChecksFailed != len(tt.want) {
				t.Errorf("checks_failed = %d, want %d", r.Summary.ChecksFailed, len(tt.want))
			}
			if diff := cmp.Diff(tt.conditions, conditions(r)); diff != "" {
				t.Errorf("conditions (-want +got):\n%s", diff)
			}
			errLine := 0
			if tt.errored {
				errLine = 1
			}
			if text := Text(r, defaults); strings.Count(text, "\n") != errLine+1+min(len(tt.want), 1)+len(tt.want) {
				t.Errorf("text has an unexpected line count:\n%s", text)
			}
		})
	}
}

func TestBuildErroredIncomplete(t *testing.T) {
	t.Parallel()
	r := mustBuild(t, "errored", defaults)
	if !r.Summary.Errored || r.Summary.Complete {
		t.Errorf("summary = %+v, want errored and incomplete", r.Summary)
	}
	if ok := mustBuild(t, "create-only", defaults); ok.Summary.Errored || !ok.Summary.Complete {
		t.Errorf("summary = %+v, want complete", ok.Summary)
	}
}

func TestBuildMetadata(t *testing.T) {
	t.Parallel()
	r := mustBuild(t, "replace-forced", defaults)
	if r.Tool != "terraform" || r.ToolVersion != "1.9.5" {
		t.Errorf("metadata = %q %q", r.Tool, r.ToolVersion)
	}
	db := r.Changes[1]
	if db.Reason != "replace_because_cannot_update" || !cmp.Equal(db.ReplacedBy, []string{"engine_version"}) {
		t.Errorf("replace = %+v", db)
	}
	if !db.Attrs[0].ForcesReplacement || db.Attrs[0].Path != "engine_version" {
		t.Errorf("first attr = %+v", db.Attrs[0])
	}
}

func TestSqueezeErrors(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"parse":           `{`,
		"resource action": `{"format_version":"1.2","resource_changes":[{"address":"a","change":{"actions":["bogus"]}}]}`,
		"drift action":    `{"format_version":"1.2","resource_drift":[{"address":"a","change":{"actions":["bogus"]}}]}`,
		"deferred action": `{"format_version":"1.2","deferred_changes":[{"reason":"x","resource_change":{"address":"a","change":{"actions":["bogus"]}}}]}`,
		"output action":   `{"format_version":"1.2","output_changes":{"o":{"actions":["bogus"]}}}`,
	}
	for name, input := range tests {
		if _, err := Squeeze([]byte(input), defaults); err == nil {
			t.Errorf("%s: Squeeze() returned no error", name)
		}
	}
}

func TestFixturesNeverLeakSensitiveValues(t *testing.T) {
	t.Parallel()
	for name, data := range fixtures(t) {
		if len(sensitiveStrings(t, data)) == 0 {
			continue
		}
		for _, o := range []Options{defaults, {IncludeNoop: true, ListDrift: true}, {NoValues: true}} {
			r, err := Squeeze(data, o)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			envelope, err := core.Envelope("tfplan", len(data), r, nil)
			if err != nil {
				t.Fatal(err)
			}
			assertNoLeak(t, name, data, r.Text, string(envelope))
		}
	}
}

func TestSensitiveFixturesHaveSecrets(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"sensitive", "replace-forced", "outputs"} {
		if len(sensitiveStrings(t, fixture(t, name))) == 0 {
			t.Errorf("%s has no sensitive values; the leak test would pass vacuously", name)
		}
	}
}

func TestRatioOnFixtures(t *testing.T) {
	t.Parallel()
	const maxTotal, maxEach = 0.15, 0.25
	var totalIn, totalOut int
	for name, data := range fixtures(t) {
		if !json.Valid(data) {
			continue
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, data); err != nil {
			t.Fatalf("compact %s: %v", name, err)
		}
		r, err := Squeeze(compact.Bytes(), defaults)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		totalIn += compact.Len()
		totalOut += len(r.Text)
		if ratio := core.Ratio(len(r.Text), compact.Len()); ratio > maxEach {
			t.Errorf("%s: ratio %.4f > %.2f", name, ratio, maxEach)
		}
	}
	if ratio := core.Ratio(totalOut, totalIn); ratio > maxTotal {
		t.Errorf("all fixtures: ratio %.4f > %.2f", ratio, maxTotal)
	}
}

func TestRatioOnLargePlan(t *testing.T) {
	t.Parallel()
	input := largePlan(t, 2000)
	r, err := Squeeze(input, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if ratio := core.Ratio(len(r.Text), len(input)); ratio > 0.15 {
		t.Errorf("ratio %.4f > 0.15", ratio)
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := `{"format_version":"1.2","terraform_version":"1.9\nerror: forged","resource_changes":[{"address":"aws_instance.a\nerror: forged","change":{"actions":["create"]}}],"output_changes":{"x\u0000\nerror: forged":{"actions":["create"]}}}`
	r, err := Squeeze([]byte(input), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(r.Text, "\n") {
		if strings.HasPrefix(line, "error:") {
			t.Errorf("input forged a line: %q\n%s", line, r.Text)
		}
	}
	if strings.ContainsRune(r.Text, 0) {
		t.Errorf("text contains NUL: %q", r.Text)
	}
}

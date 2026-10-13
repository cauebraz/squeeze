package tfplan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
)

var defaults = DefaultOptions()

func build(t testing.TB, input []byte, o Options) (Report, error) {
	t.Helper()
	p, err := Parse(input)
	if err != nil {
		return Report{}, err
	}
	return Build(p, o)
}

func squeezeFixture(t *testing.T, name string, o Options) Report {
	t.Helper()
	r, err := build(t, fixture(t, name), o)
	if err != nil {
		t.Fatalf("Squeeze(%s) error = %v", name, err)
	}
	return r
}

func addresses(cs []ResourceChange) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Action.Symbol()+" "+c.Address)
	}
	return out
}

func TestSqueezeOrdering(t *testing.T) {
	t.Parallel()
	plan := &Plan{FormatVersion: "1.2", ResourceChanges: []PlanResource{
		{Address: "b.create", Change: PlanChange{Actions: []string{"create"}}},
		{Address: "a.create", Change: PlanChange{Actions: []string{"create"}}},
		{Address: "z.update", Change: PlanChange{Actions: []string{"update"}, Before: map[string]any{}, After: map[string]any{}}},
		{Address: "y.read", Change: PlanChange{Actions: []string{"read"}}},
		{Address: "x.import", Change: PlanChange{Actions: []string{"no-op"}, Importing: &PlanImporting{ID: "x"}}},
		{Address: "w.cbd", Change: PlanChange{Actions: []string{"create", "delete"}}},
		{Address: "v.replace", Change: PlanChange{Actions: []string{"delete", "create"}}},
		{Address: "u.delete", Change: PlanChange{Actions: []string{"delete"}}},
		{Address: "t.forget", Change: PlanChange{Actions: []string{"forget"}}},
		{Address: "u.delete", Deposed: "b", Change: PlanChange{Actions: []string{"delete"}}},
		{Address: "u.delete", Deposed: "a", Change: PlanChange{Actions: []string{"delete"}}},
	}}
	r, err := Build(plan, defaults)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"- u.delete", "- u.delete", "- u.delete",
		"-/+ v.replace", "+/- w.cbd", "- t.forget", "~ z.update",
		"+ a.create", "+ b.create", "<= y.read", ">> x.import",
	}
	if got := addresses(r.Changes); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order =\n%q\nwant\n%q", got, want)
	}
	if r.Changes[0].Deposed != "" || r.Changes[1].Deposed != "a" || r.Changes[2].Deposed != "b" {
		t.Errorf("deposed order = %q, %q, %q", r.Changes[0].Deposed, r.Changes[1].Deposed, r.Changes[2].Deposed)
	}
	wantSummary := Summary{Add: 2, Change: 1, Destroy: 3, Replace: 2, Read: 1, Import: 1, Forget: 1, Complete: true}
	if r.Summary != wantSummary {
		t.Errorf("summary = %+v, want %+v", r.Summary, wantSummary)
	}
}

func TestSqueezeTruncation(t *testing.T) {
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
	tests := []struct {
		name         string
		o            Options
		wantChanges  int
		wantOmitted  int
		wantAttrs    int
		wantAttrsCut int
	}{
		{"defaults", defaults, 5, 0, 8, 2},
		{"limits", Options{MaxAttrs: 3, MaxResources: 2}, 2, 3, 3, 7},
		{"zero means all", Options{}, 5, 0, 10, 0},
		{"exact limits", Options{MaxAttrs: 10, MaxResources: 5}, 5, 0, 10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, err := Build(plan, tt.o)
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Changes) != tt.wantChanges || r.Omitted != tt.wantOmitted {
				t.Errorf("changes = %d, omitted = %d; want %d, %d", len(r.Changes), r.Omitted, tt.wantChanges, tt.wantOmitted)
			}
			c := r.Changes[0]
			if len(c.Attrs) != tt.wantAttrs || c.AttrsOmitted != tt.wantAttrsCut {
				t.Errorf("attrs = %d, omitted = %d; want %d, %d", len(c.Attrs), c.AttrsOmitted, tt.wantAttrs, tt.wantAttrsCut)
			}
			if r.Summary.Change != 5 {
				t.Errorf("summary.Change = %d, want 5 regardless of truncation", r.Summary.Change)
			}
		})
	}
}

func TestSqueezeNoopVisibility(t *testing.T) {
	t.Parallel()
	hidden := squeezeFixture(t, "noop-only.json", defaults)
	if len(hidden.Changes) != 0 || len(hidden.Outputs) != 0 {
		t.Errorf("noop hidden: changes = %v, outputs = %v", hidden.Changes, hidden.Outputs)
	}
	if hidden.Summary.Noop != 2 || hidden.Summary.Changed() {
		t.Errorf("summary = %+v", hidden.Summary)
	}
	all := defaults
	all.IncludeNoop = true
	shown := squeezeFixture(t, "noop-only.json", all)
	if len(shown.Changes) != 2 || len(shown.Outputs) != 1 {
		t.Errorf("noop shown: changes = %d, outputs = %d; want 2, 1", len(shown.Changes), len(shown.Outputs))
	}
	if shown.Summary.Outputs != 0 {
		t.Errorf("summary.Outputs = %d, want 0 for no-op outputs", shown.Summary.Outputs)
	}
}

func TestSqueezeMovedNoopStaysVisible(t *testing.T) {
	t.Parallel()
	r := squeezeFixture(t, "moved.json", defaults)
	want := []string{"- aws_s3_bucket.archive", "~ module.storage.aws_s3_bucket.this[0]", "= aws_iam_role.app"}
	if got := addresses(r.Changes); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("changes = %q, want %q", got, want)
	}
	if r.Summary.Move != 2 || r.Summary.Forget != 1 || r.Summary.Change != 1 || r.Summary.Noop != 1 {
		t.Errorf("summary = %+v", r.Summary)
	}
	if r.Changes[2].PreviousAddress != "aws_iam_role.legacy" {
		t.Errorf("previous address = %q", r.Changes[2].PreviousAddress)
	}
}

func TestSqueezeAttrsOnlyForUpdatesAndReplaces(t *testing.T) {
	t.Parallel()
	obj := map[string]any{"a": "1"}
	obj2 := map[string]any{"a": "2"}
	for _, actions := range [][]string{{"create"}, {"delete"}, {"read"}, {"forget"}, {"no-op"}} {
		plan := &Plan{FormatVersion: "1.2", ResourceChanges: []PlanResource{
			{Address: "r.x", Change: PlanChange{Actions: actions, Before: obj, After: obj2}},
		}}
		r, err := Build(plan, Options{IncludeNoop: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Changes[0].Attrs) != 0 {
			t.Errorf("actions %q produced attrs %+v", actions, r.Changes[0].Attrs)
		}
	}
	for _, actions := range [][]string{{"update"}, {"delete", "create"}, {"create", "delete"}} {
		plan := &Plan{FormatVersion: "1.2", ResourceChanges: []PlanResource{
			{Address: "r.x", Change: PlanChange{Actions: actions, Before: obj, After: obj2}},
		}}
		r, err := Build(plan, defaults)
		if err != nil {
			t.Fatal(err)
		}
		if len(r.Changes[0].Attrs) != 1 {
			t.Errorf("actions %q attrs = %+v, want 1", actions, r.Changes[0].Attrs)
		}
	}
}

func TestSqueezeDrift(t *testing.T) {
	t.Parallel()
	r := squeezeFixture(t, "drift.json", defaults)
	if r.Summary.Drift != 2 || len(r.Drift) != 2 {
		t.Fatalf("drift = %d / %d, want 2", r.Summary.Drift, len(r.Drift))
	}
	if r.Drift[0].Action != Delete || r.Drift[1].Address != "aws_s3_bucket.assets" {
		t.Errorf("drift order = %q", addresses(r.Drift))
	}
	if len(r.Drift[1].Attrs) != 1 || r.Drift[1].Attrs[0].Path != "tags.Owner" {
		t.Errorf("drift attrs = %+v", r.Drift[1].Attrs)
	}
}

func TestSqueezeOutputs(t *testing.T) {
	t.Parallel()
	r := squeezeFixture(t, "outputs.json", defaults)
	want := []OutputChange{
		{Name: "bucket_name", Action: Delete},
		{Name: "db_endpoint", Action: Create, Unknown: true},
		{Name: "db_password", Action: Update, Sensitive: true},
	}
	if len(r.Outputs) != len(want) {
		t.Fatalf("outputs = %+v", r.Outputs)
	}
	for i := range want {
		if r.Outputs[i] != want[i] {
			t.Errorf("outputs[%d] = %+v, want %+v", i, r.Outputs[i], want[i])
		}
	}
	if r.Summary.Outputs != 3 {
		t.Errorf("summary.Outputs = %d, want 3", r.Summary.Outputs)
	}
}

func TestSqueezeImport(t *testing.T) {
	t.Parallel()
	r := squeezeFixture(t, "import.json", defaults)
	if got := addresses(r.Changes); strings.Join(got, ",") != "~ aws_iam_role.ci,>> aws_s3_bucket.legacy" {
		t.Errorf("changes = %q", got)
	}
	if !r.Changes[0].Importing || r.Summary.Import != 2 || r.Summary.Change != 1 {
		t.Errorf("import summary = %+v, importing = %v", r.Summary, r.Changes[0].Importing)
	}
}

func TestSqueezeErroredIncomplete(t *testing.T) {
	t.Parallel()
	r := squeezeFixture(t, "errored.json", defaults)
	if !r.Summary.Errored || r.Summary.Complete {
		t.Errorf("summary = %+v, want errored and incomplete", r.Summary)
	}
	ok := squeezeFixture(t, "create-only.json", defaults)
	if ok.Summary.Errored || !ok.Summary.Complete {
		t.Errorf("summary = %+v, want complete", ok.Summary)
	}
}

func TestSqueezeMetadata(t *testing.T) {
	t.Parallel()
	input := fixture(t, "replace-forced.json")
	r, err := build(t, input, defaults)
	if err != nil {
		t.Fatal(err)
	}
	if r.Tool != "terraform" || r.ToolVersion != "1.9.5" {
		t.Errorf("metadata = %q %q", r.Tool, r.ToolVersion)
	}
	db := r.Changes[1]
	if db.Reason != "replace_because_cannot_update" || strings.Join(db.ReplacedBy, ",") != "engine_version" {
		t.Errorf("replace = %+v", db)
	}
	if !db.Attrs[0].ForcesReplacement || db.Attrs[0].Path != "engine_version" {
		t.Errorf("first attr = %+v, want engine_version forcing replacement", db.Attrs[0])
	}
}

func TestSqueezeErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
	}{
		{"parse", `{`},
		{"resource action", `{"format_version":"1.2","resource_changes":[{"address":"a","change":{"actions":["bogus"]}}]}`},
		{"drift action", `{"format_version":"1.2","resource_changes":[],"resource_drift":[{"address":"a","change":{"actions":["bogus"]}}]}`},
		{"output action", `{"format_version":"1.2","resource_changes":[],"output_changes":{"o":{"actions":["bogus"]}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Squeeze([]byte(tt.input), defaults); err == nil {
				t.Error("Squeeze() returned no error")
			}
		})
	}
}

var leakedSecrets = []string{"hunter2", "correct-horse", "k-old-123", "k-new-456", "old-secret", "new-secret"}

func TestFixturesNeverLeakSensitiveValues(t *testing.T) {
	t.Parallel()
	for name, data := range fixtures(t) {
		for _, values := range []bool{true, false} {
			o := Options{NoValues: !values, IncludeNoop: true}
			r, err := build(t, data, o)
			if err != nil {
				t.Fatalf("Squeeze(%s) error = %v", name, err)
			}
			text := Text(r, Options{ListDrift: true})
			js, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range leakedSecrets {
				if strings.Contains(text, s) || bytes.Contains(js, []byte(s)) {
					t.Errorf("%s (values=%v) leaks %q", name, values, s)
				}
			}
		}
	}
}

func TestRatioOnFixtures(t *testing.T) {
	t.Parallel()
	const maxTotal, maxEach = 0.15, 0.25
	var totalIn, totalOut int
	for name, data := range fixtures(t) {
		var compact bytes.Buffer
		if err := json.Compact(&compact, data); err != nil {
			t.Fatalf("compact %s: %v", name, err)
		}
		r, err := build(t, compact.Bytes(), defaults)
		if err != nil {
			t.Fatalf("Squeeze(%s) error = %v", name, err)
		}
		text := Text(r, Options{})
		totalIn += compact.Len()
		totalOut += len(text)
		if ratio := core.Ratio(len(text), compact.Len()); ratio > maxEach {
			t.Errorf("%s: ratio %.4f > %.2f (%d of %d bytes)", name, ratio, maxEach, len(text), compact.Len())
		}
	}
	if ratio := core.Ratio(totalOut, totalIn); ratio > maxTotal {
		t.Errorf("all fixtures: ratio %.4f > %.2f (%d of %d bytes)", ratio, maxTotal, totalOut, totalIn)
	}
}

func TestRatioOnLargePlan(t *testing.T) {
	t.Parallel()
	input := largePlan(t, 2000)
	r, err := build(t, input, defaults)
	if err != nil {
		t.Fatal(err)
	}
	text := Text(r, Options{})
	if ratio := core.Ratio(len(text), len(input)); ratio > 0.15 {
		t.Errorf("ratio %.4f > 0.15 (%d of %d bytes)", ratio, len(text), len(input))
	}
}

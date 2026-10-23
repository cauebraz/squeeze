package tfplan

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/cauebraz/squeeze/internal/tfdiag"
)

func TestHeader(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		r    report
		want string
	}{
		"no changes": {
			report{Tool: "terraform", ToolVersion: "1.9.5", Summary: summary{Complete: true}},
			"terraform 1.9.5 plan: no changes",
		},
		"no version": {
			report{Tool: "terraform", Summary: summary{Complete: true}},
			"terraform plan: no changes",
		},
		"base counts always shown": {
			report{Tool: "terraform", Summary: summary{Add: 1, Complete: true}},
			"terraform plan: 1 to add, 0 to change, 0 to destroy",
		},
		"import leads like terraform": {
			report{Tool: "terraform", Summary: summary{Import: 2, Change: 1, Complete: true}},
			"terraform plan: 2 to import, 0 to add, 1 to change, 0 to destroy",
		},
		"extra counts in order": {
			report{Tool: "terraform", Summary: summary{Add: 1, Destroy: 1, Replace: 1, Forget: 5, Move: 6, Read: 4, Complete: true}},
			"terraform plan: 1 to add, 0 to change, 1 to destroy, 1 to replace, 5 to forget, 6 to move, 4 to read",
		},
		"side effects and incomplete": {
			report{Tool: "terraform", Summary: summary{Drift: 2, Deferred: 3, Actions: 1, Outputs: 1}},
			"terraform plan: no changes; 2 drifted; 3 deferred; 1 action to invoke; 1 output changed; incomplete",
		},
		"deferred actions": {
			report{Tool: "terraform", Summary: summary{Deferred: 1, DeferredActions: 1, Actions: 1, Complete: true}},
			"terraform plan: no changes; 1 deferred; 1 deferred action; 1 action to invoke",
		},
		"plurals": {
			report{Tool: "terraform", Summary: summary{Actions: 2, Outputs: 2, Complete: true}},
			"terraform plan: no changes; 2 actions to invoke; 2 outputs changed",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := header(tt.r); got != tt.want {
				t.Errorf("header() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestAttrLine(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 47)
	tests := map[string]struct {
		a    attrChange
		want string
	}{
		"modified":                {attrChange{Path: "a", Before: sv("x"), After: sv("y")}, `~ a: "x" -> "y"`},
		"path only":               {attrChange{Path: "a"}, "~ a"},
		"modified one side":       {attrChange{Path: "a", Before: sv("x")}, "~ a"},
		"added":                   {attrChange{Path: "a", Kind: attrAdded, After: sv("y")}, `+ a: "y"`},
		"added without value":     {attrChange{Path: "a", Kind: attrAdded}, "+ a"},
		"removed":                 {attrChange{Path: "a", Kind: attrRemoved, Before: sv("x")}, `- a: "x"`},
		"removed without value":   {attrChange{Path: "a", Kind: attrRemoved}, "- a"},
		"forces replacement":      {attrChange{Path: "a", Before: num("1"), After: num("2"), ForcesReplacement: true}, "! a: 1 -> 2"},
		"unknown":                 {attrChange{Path: "a", Unknown: true}, "~ a: (known after apply)"},
		"unknown added":           {attrChange{Path: "a", Kind: attrAdded, Unknown: true}, "+ a: (known after apply)"},
		"unknown with before":     {attrChange{Path: "a", Before: sv("x"), Unknown: true}, `~ a: "x" -> (known after apply)`},
		"sensitive ignores model": {attrChange{Path: "a", Sensitive: true, Before: sv("leak"), After: sv("leak")}, "~ a: (sensitive)"},
		"sensitive unknown":       {attrChange{Path: "a", Sensitive: true, Unknown: true}, "~ a: (sensitive, known after apply)"},
		"value at 48 bytes":       {attrChange{Path: "a", Before: sv("x"), After: sv(long[:46])}, `~ a: "x" -> "` + long[:46] + `"`},
		"value over 48 bytes":     {attrChange{Path: "a", Before: sv("x"), After: sv(long)}, "~ a"},
		"null":                    {attrChange{Path: "a", Before: sv(nil), After: sv(false)}, "~ a: null -> false"},
		"float":                   {attrChange{Path: "a", Before: &scalar{Value: 1.5}, After: &scalar{Value: 2.0}}, "~ a: 1.5 -> 2"},
		"non-scalar value":        {attrChange{Path: "a", Before: &scalar{Value: []any{}}, After: sv("x")}, "~ a"},
		"reordered":               {attrChange{Path: "a", Reordered: true}, "~ a: order changed"},
		"reordered forces":        {attrChange{Path: "a", Reordered: true, ForcesReplacement: true}, "! a: order changed"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := attrLine(tt.a); got != tt.want {
				t.Errorf("attrLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNotes(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		c    resourceChange
		want []string
	}{
		"none":                      {resourceChange{Address: "a", Action: actionUpdate}, nil},
		"same previous address":     {resourceChange{Address: "a", PreviousAddress: "a"}, nil},
		"import action has no note": {resourceChange{Action: actionImport, Importing: true}, nil},
		"all notes in order": {
			resourceChange{
				Address: "b", PreviousAddress: "a", Deposed: "d1", Action: actionUpdate, Importing: true,
				ReplacedBy: []string{"x", "y"}, Reason: "replace_because_tainted", Irrelevant: true,
			},
			[]string{"deposed object d1", "moved from a", "import", "replacement forced by x, y", "replace_because_tainted", "does not affect this plan"},
		},
		"generated config":   {resourceChange{Action: actionCreate, Importing: true, GeneratedLines: 3, GeneratedConfig: "a\nb\nc"}, []string{"import", "3 lines of generated config in --output json"}},
		"generated one line": {resourceChange{Action: actionImport, Importing: true, GeneratedLines: 1}, []string{"1 line of generated config"}},
		"forget":             {resourceChange{Action: actionForget}, []string{"removed from state, not destroyed"}},
		"forget then create": {resourceChange{Action: actionForgetThenCreate}, []string{"existing object not destroyed"}},
		"create then forget": {resourceChange{Action: actionCreateThenForget}, []string{"existing object not destroyed"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, notes(tt.c)); diff != "" {
				t.Errorf("notes() (-want +got):\n%s", diff)
			}
		})
	}
}

func TestText(t *testing.T) {
	t.Parallel()
	r := report{
		Tool:        "terraform",
		ToolVersion: "1.9.5",
		Summary:     summary{Add: 1, Destroy: 2, Replace: 1, Drift: 2, Deferred: 1, DeferredActions: 2, Actions: 1, Outputs: 2, Errored: true, Complete: false},
		Changes: []resourceChange{
			{Address: "aws_instance.old", Action: actionDelete},
			{
				Address: "aws_db_instance.main", Action: actionReplace, ReplacedBy: []string{"engine_version"},
				Attrs: []attrChange{
					{Path: "engine_version", Before: sv("15"), After: sv("16"), ForcesReplacement: true},
					{Path: "password", Sensitive: true},
				},
				AttrsOmitted: 1,
			},
		},
		Omitted: 12,
		Drift: []resourceChange{
			{Address: "aws_s3_bucket.a", Action: actionUpdate, Attrs: []attrChange{{Path: "tags.Owner", Kind: attrRemoved, Before: sv("me")}}},
			{Address: "aws_instance.b", Action: actionDelete},
		},
		DriftOmitted: 3,
		DriftHidden:  1,
		Deferred:     []deferredChange{{Address: "aws_subnet.x[\"*\"]", Action: actionCreate, Reason: "instance_count_unknown"}},
		Actions:      []tfdiag.ActionInvocation{{Address: "action.a.n", Event: "AfterCreate", Resource: "aws_instance.web"}, {Address: "action.a.m", Invoked: true}, {Address: "action.a.o", Event: "BeforeDestroy"}, {Address: "action.a.p"}},
		DeferredActions: []deferredAction{
			{tfdiag.ActionInvocation{Address: "action.a.q", Event: "AfterUpdate", Resource: "aws_subnet.x"}, "deferred_prereq"},
			{tfdiag.ActionInvocation{Address: "action.a.r"}, "absent_prereq"},
		},
		Outputs: []outputChange{
			{Name: "endpoint", Action: actionCreate, Unknown: true},
			{Name: "password", Action: actionUpdate, Sensitive: true, Unknown: true},
			{Name: "region", Action: actionDelete},
		},
	}
	common := `... 12 more resources (--max-resources 0)
`
	tail := `... 1 drifted resource not affecting this plan (--all)
deferred:
+ aws_subnet.x["*"] (instance_count_unknown)
actions: action.a.n (AfterCreate aws_instance.web), action.a.m (invoked), action.a.o (BeforeDestroy), action.a.p
deferred actions: action.a.q (AfterUpdate aws_subnet.x; deferred_prereq), action.a.r (absent_prereq)
outputs: + endpoint (known after apply), ~ password (sensitive), - region
`
	head := `plan errored
terraform 1.9.5 plan: 1 to add, 0 to change, 2 to destroy, 1 to replace; 2 drifted; 1 deferred; 2 deferred actions; 1 action to invoke; 2 outputs changed; incomplete
- aws_instance.old
-/+ aws_db_instance.main (replacement forced by engine_version)
  ! engine_version: "15" -> "16"
  ~ password: (sensitive)
  ... 1 more attribute (--max-attrs 0)
`
	tests := map[string]struct {
		o    options
		want string
	}{
		"drift counted": {options{}, head + common + "... 5 drifted resources (--drift)\n" + tail},
		"drift listed": {options{ListDrift: true}, head + common + `drift:
~ aws_s3_bucket.a
  - tags.Owner: "me"
- aws_instance.b
... 3 more drifted resources (--max-resources 0)
` + tail},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, text(r, tt.o)); diff != "" {
				t.Errorf("Text() (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTextSingulars(t *testing.T) {
	t.Parallel()
	r := report{Tool: "terraform", Summary: summary{Complete: true}, Omitted: 1, Drift: []resourceChange{{Address: "a", Action: actionUpdate}}}
	got := text(r, options{})
	for _, s := range []string{"... 1 more resource (--max-resources 0)\n", "... 1 drifted resource (--drift)\n"} {
		if !strings.Contains(got, s) {
			t.Errorf("Text() lacks %q:\n%s", s, got)
		}
	}
}

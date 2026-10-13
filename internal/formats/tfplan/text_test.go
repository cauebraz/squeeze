package tfplan

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestHeader(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		r    Report
		want string
	}{
		"no changes": {
			Report{Tool: "terraform", ToolVersion: "1.9.5", Summary: Summary{Complete: true}},
			"terraform 1.9.5 plan: no changes",
		},
		"no version": {
			Report{Tool: "terraform", Summary: Summary{Complete: true}},
			"terraform plan: no changes",
		},
		"base counts always shown": {
			Report{Tool: "terraform", Summary: Summary{Add: 1, Complete: true}},
			"terraform plan: 1 to add, 0 to change, 0 to destroy",
		},
		"import leads like terraform": {
			Report{Tool: "terraform", Summary: Summary{Import: 2, Change: 1, Complete: true}},
			"terraform plan: 2 to import, 0 to add, 1 to change, 0 to destroy",
		},
		"extra counts in order": {
			Report{Tool: "terraform", Summary: Summary{Add: 1, Destroy: 1, Replace: 1, Forget: 5, Move: 6, Read: 4, Complete: true}},
			"terraform plan: 1 to add, 0 to change, 1 to destroy, 1 to replace, 5 to forget, 6 to move, 4 to read",
		},
		"side effects and incomplete": {
			Report{Tool: "terraform", Summary: Summary{Drift: 2, Deferred: 3, Actions: 1, Outputs: 1}},
			"terraform plan: no changes; 2 drifted; 3 deferred; 1 action to invoke; 1 output changed; incomplete",
		},
		"plurals": {
			Report{Tool: "terraform", Summary: Summary{Actions: 2, Outputs: 2, Complete: true}},
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
		a    AttrChange
		want string
	}{
		"modified":                {AttrChange{Path: "a", Before: sv("x"), After: sv("y")}, `~ a: "x" -> "y"`},
		"path only":               {AttrChange{Path: "a"}, "~ a"},
		"modified one side":       {AttrChange{Path: "a", Before: sv("x")}, "~ a"},
		"added":                   {AttrChange{Path: "a", Kind: Added, After: sv("y")}, `+ a: "y"`},
		"added without value":     {AttrChange{Path: "a", Kind: Added}, "+ a"},
		"removed":                 {AttrChange{Path: "a", Kind: Removed, Before: sv("x")}, `- a: "x"`},
		"removed without value":   {AttrChange{Path: "a", Kind: Removed}, "- a"},
		"forces replacement":      {AttrChange{Path: "a", Before: num("1"), After: num("2"), ForcesReplacement: true}, "! a: 1 -> 2"},
		"unknown":                 {AttrChange{Path: "a", Unknown: true}, "~ a: (known after apply)"},
		"unknown added":           {AttrChange{Path: "a", Kind: Added, Unknown: true}, "+ a: (known after apply)"},
		"unknown with before":     {AttrChange{Path: "a", Before: sv("x"), Unknown: true}, `~ a: "x" -> (known after apply)`},
		"sensitive ignores model": {AttrChange{Path: "a", Sensitive: true, Before: sv("leak"), After: sv("leak")}, "~ a: (sensitive)"},
		"sensitive unknown":       {AttrChange{Path: "a", Sensitive: true, Unknown: true}, "~ a: (sensitive, known after apply)"},
		"value at 48 bytes":       {AttrChange{Path: "a", Before: sv("x"), After: sv(long[:46])}, `~ a: "x" -> "` + long[:46] + `"`},
		"value over 48 bytes":     {AttrChange{Path: "a", Before: sv("x"), After: sv(long)}, "~ a"},
		"null":                    {AttrChange{Path: "a", Before: sv(nil), After: sv(false)}, "~ a: null -> false"},
		"float":                   {AttrChange{Path: "a", Before: &Scalar{Value: 1.5}, After: &Scalar{Value: 2.0}}, "~ a: 1.5 -> 2"},
		"non-scalar value":        {AttrChange{Path: "a", Before: &Scalar{Value: []any{}}, After: sv("x")}, "~ a"},
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
		c    ResourceChange
		want []string
	}{
		"none":                      {ResourceChange{Address: "a", Action: Update}, nil},
		"same previous address":     {ResourceChange{Address: "a", PreviousAddress: "a"}, nil},
		"import action has no note": {ResourceChange{Action: Import, Importing: true}, nil},
		"all notes in order": {
			ResourceChange{
				Address: "b", PreviousAddress: "a", Deposed: "d1", Action: Update, Importing: true,
				ReplacedBy: []string{"x", "y"}, Reason: "replace_because_tainted", Irrelevant: true,
			},
			[]string{"deposed object d1", "moved from a", "import", "replacement forced by x, y", "replace_because_tainted", "does not affect this plan"},
		},
		"forget":             {ResourceChange{Action: Forget}, []string{"removed from state, not destroyed"}},
		"forget then create": {ResourceChange{Action: ForgetThenCreate}, []string{"existing object not destroyed"}},
		"create then forget": {ResourceChange{Action: CreateThenForget}, []string{"existing object not destroyed"}},
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
	r := Report{
		Tool:        "terraform",
		ToolVersion: "1.9.5",
		Summary:     Summary{Add: 1, Destroy: 2, Replace: 1, Drift: 2, Deferred: 1, Actions: 1, Outputs: 2, Errored: true, Complete: false},
		Changes: []ResourceChange{
			{Address: "aws_instance.old", Action: Delete},
			{
				Address: "aws_db_instance.main", Action: Replace, ReplacedBy: []string{"engine_version"},
				Attrs: []AttrChange{
					{Path: "engine_version", Before: sv("15"), After: sv("16"), ForcesReplacement: true},
					{Path: "password", Sensitive: true},
				},
				AttrsOmitted: 1,
			},
		},
		Omitted: 12,
		Drift: []ResourceChange{
			{Address: "aws_s3_bucket.a", Action: Update, Attrs: []AttrChange{{Path: "tags.Owner", Kind: Removed, Before: sv("me")}}},
			{Address: "aws_instance.b", Action: Delete},
		},
		DriftOmitted: 1,
		Deferred:     []DeferredChange{{Address: "aws_subnet.x[\"*\"]", Action: Create, Reason: "instance_count_unknown"}},
		Actions:      []ActionInvocation{{Address: "action.a.n", Event: "AfterCreate", Resource: "aws_instance.web"}, {Address: "action.a.m", Invoked: true}, {Address: "action.a.o", Event: "BeforeDestroy"}, {Address: "action.a.p"}},
		Outputs: []OutputChange{
			{Name: "endpoint", Action: Create, Unknown: true},
			{Name: "password", Action: Update, Sensitive: true, Unknown: true},
			{Name: "region", Action: Delete},
		},
	}
	common := `... 12 more resources (--max-resources)
`
	tail := `... 1 drifted resource not affecting this plan (--all)
deferred:
+ aws_subnet.x["*"] (instance_count_unknown)
actions: action.a.n (AfterCreate aws_instance.web), action.a.m (invoked), action.a.o (BeforeDestroy), action.a.p
outputs: + endpoint (known after apply), ~ password (sensitive), - region
`
	head := `plan errored
terraform 1.9.5 plan: 1 to add, 0 to change, 2 to destroy, 1 to replace; 2 drifted; 1 deferred; 1 action to invoke; 2 outputs changed; incomplete
- aws_instance.old
-/+ aws_db_instance.main (replacement forced by engine_version)
  ! engine_version: "15" -> "16"
  ~ password: (sensitive)
  ... 1 more attribute (--max-attrs)
`
	tests := map[string]struct {
		o    Options
		want string
	}{
		"drift counted": {Options{}, head + common + "drift: 2 resources (--drift to list)\n" + tail},
		"drift listed": {Options{ListDrift: true}, head + common + `drift:
~ aws_s3_bucket.a
  - tags.Owner: "me"
- aws_instance.b
` + tail},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if diff := cmp.Diff(tt.want, Text(r, tt.o)); diff != "" {
				t.Errorf("Text() (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTextSingulars(t *testing.T) {
	t.Parallel()
	r := Report{Tool: "terraform", Summary: Summary{Complete: true}, Omitted: 1, Drift: []ResourceChange{{Address: "a", Action: Update}}}
	got := Text(r, Options{})
	for _, s := range []string{"... 1 more resource (--max-resources)\n", "drift: 1 resource (--drift to list)\n"} {
		if !strings.Contains(got, s) {
			t.Errorf("Text() lacks %q:\n%s", s, got)
		}
	}
}

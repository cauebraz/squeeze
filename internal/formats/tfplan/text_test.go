package tfplan

import (
	"strings"
	"testing"
)

func TestHeader(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		r    Report
		want string
	}{
		{
			name: "no changes",
			r:    Report{Tool: "terraform", ToolVersion: "1.9.5", Summary: Summary{Complete: true}},
			want: "terraform 1.9.5 plan: no changes",
		},
		{
			name: "no version",
			r:    Report{Tool: "terraform", Summary: Summary{Complete: true}},
			want: "terraform plan: no changes",
		},
		{
			name: "base counts always shown",
			r:    Report{Tool: "terraform", Summary: Summary{Add: 1, Complete: true}},
			want: "terraform plan: 1 to add, 0 to change, 0 to destroy",
		},
		{
			name: "optional counts in order",
			r: Report{Tool: "terraform", Summary: Summary{
				Change: 2, Replace: 1, Import: 3, Read: 4, Forget: 5, Move: 6, Complete: true,
			}},
			want: "terraform plan: 0 to add, 2 to change, 0 to destroy, 1 to replace, 3 to import, 4 to read, 5 to forget, 6 to move",
		},
		{
			name: "drift one output incomplete",
			r:    Report{Tool: "terraform", Summary: Summary{Drift: 2, Outputs: 1}},
			want: "terraform plan: no changes; 2 drifted; 1 output changed; incomplete",
		},
		{
			name: "outputs plural",
			r:    Report{Tool: "terraform", Summary: Summary{Outputs: 2, Complete: true}},
			want: "terraform plan: no changes; 2 outputs changed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := header(tt.r); got != tt.want {
				t.Errorf("header() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestAttrLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		a    AttrChange
		want string
	}{
		{"values", AttrChange{Path: "a", Before: `"x"`, After: `"y"`}, `~ a: "x" -> "y"`},
		{"path only", AttrChange{Path: "a"}, "~ a"},
		{"one side only", AttrChange{Path: "a", Before: `"x"`}, "~ a"},
		{"forces replacement", AttrChange{Path: "a", Before: "1", After: "2", ForcesReplacement: true}, "! a: 1 -> 2"},
		{"unknown", AttrChange{Path: "a", Unknown: true}, "~ a: (known after apply)"},
		{"unknown with before", AttrChange{Path: "a", Before: `"x"`, Unknown: true}, `~ a: "x" -> (known after apply)`},
		{"sensitive", AttrChange{Path: "a", Sensitive: true, Before: "leak", After: "leak"}, "~ a: (sensitive)"},
		{"sensitive unknown", AttrChange{Path: "a", Sensitive: true, Unknown: true}, "~ a: (sensitive, known after apply)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := attrLine(tt.a); got != tt.want {
				t.Errorf("attrLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNotes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		c    ResourceChange
		want []string
	}{
		{"none", ResourceChange{Address: "a", Action: Update}, nil},
		{"same previous address", ResourceChange{Address: "a", PreviousAddress: "a"}, nil},
		{"import action has no note", ResourceChange{Action: Import, Importing: true}, nil},
		{
			name: "all notes in order",
			c: ResourceChange{
				Address: "b", PreviousAddress: "a", Deposed: "d1", Action: Update, Importing: true,
				ReplacedBy: []string{"x", "y"}, Reason: "replace_because_tainted",
			},
			want: []string{"deposed object d1", "moved from a", "import", "replacement forced by x, y", "replace_because_tainted"},
		},
		{"forget", ResourceChange{Action: Forget}, []string{"removed from state, not destroyed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := notes(tt.c)
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("notes() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestText(t *testing.T) {
	t.Parallel()
	r := Report{
		Tool:        "terraform",
		ToolVersion: "1.9.5",
		Summary:     Summary{Destroy: 1, Replace: 1, Drift: 2, Outputs: 2, Errored: true, Complete: true},
		Changes: []ResourceChange{
			{Address: "aws_instance.old", Action: Delete},
			{
				Address: "aws_db_instance.main", Action: Replace, ReplacedBy: []string{"engine_version"},
				Attrs: []AttrChange{
					{Path: "engine_version", Before: `"15"`, After: `"16"`, ForcesReplacement: true},
					{Path: "password", Sensitive: true},
				},
				AttrsOmitted: 3,
			},
		},
		Omitted: 12,
		Drift: []ResourceChange{
			{Address: "aws_s3_bucket.a", Action: Update, Attrs: []AttrChange{{Path: "tags.Owner"}}},
			{Address: "aws_instance.b", Action: Delete},
		},
		Outputs: []OutputChange{
			{Name: "endpoint", Action: Create, Unknown: true},
			{Name: "password", Action: Update, Sensitive: true, Unknown: true},
			{Name: "region", Action: Delete},
		},
	}
	tests := []struct {
		name string
		o    Options
		want string
	}{
		{
			name: "drift counted",
			want: `plan errored
terraform 1.9.5 plan: 0 to add, 0 to change, 1 to destroy, 1 to replace; 2 drifted; 2 outputs changed
- aws_instance.old
-/+ aws_db_instance.main (replacement forced by engine_version)
  ! engine_version: "15" -> "16"
  ~ password: (sensitive)
  ... 3 more attributes (--max-attrs)
... 12 more resources (--max-resources)
drift: 2 resources (--drift to list)
outputs: + endpoint (known after apply), ~ password (sensitive), - region
`,
		},
		{
			name: "drift listed",
			o:    Options{ListDrift: true},
			want: `plan errored
terraform 1.9.5 plan: 0 to add, 0 to change, 1 to destroy, 1 to replace; 2 drifted; 2 outputs changed
- aws_instance.old
-/+ aws_db_instance.main (replacement forced by engine_version)
  ! engine_version: "15" -> "16"
  ~ password: (sensitive)
  ... 3 more attributes (--max-attrs)
... 12 more resources (--max-resources)
drift:
~ aws_s3_bucket.a
  ~ tags.Owner
- aws_instance.b
outputs: + endpoint (known after apply), ~ password (sensitive), - region
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Text(r, tt.o); got != tt.want {
				t.Errorf("Text() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestTextSingleDriftResource(t *testing.T) {
	t.Parallel()
	r := Report{Tool: "terraform", Summary: Summary{Drift: 1, Complete: true}, Drift: []ResourceChange{{Address: "a", Action: Update}}}
	if got := Text(r, Options{}); !strings.Contains(got, "drift: 1 resource (--drift to list)\n") {
		t.Errorf("Text() = %q, want singular drift line", got)
	}
}

func TestTextIsDeterministic(t *testing.T) {
	t.Parallel()
	r := Report{Tool: "terraform", Summary: Summary{Change: 1, Complete: true}, Changes: []ResourceChange{
		{Address: "a", Action: Update, Attrs: []AttrChange{{Path: "x"}, {Path: "y"}}},
	}}
	first := Text(r, Options{})
	for range 20 {
		if got := Text(r, Options{}); got != first {
			t.Fatalf("Text() changed between runs:\n%s\n%s", first, got)
		}
	}
}

func TestTextHasNoControlCharacters(t *testing.T) {
	t.Parallel()
	r := Report{Tool: "terraform", Summary: Summary{Add: 1, Complete: true}, Changes: []ResourceChange{{Address: "a", Action: Create}}}
	for _, ch := range Text(r, Options{}) {
		if ch < 0x20 && ch != '\n' {
			t.Fatalf("Text() contains control character %q", ch)
		}
	}
}

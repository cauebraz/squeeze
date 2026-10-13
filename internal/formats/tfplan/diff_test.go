package tfplan

import (
	"encoding/json"
	"strings"
	"testing"
)

type diffCase struct {
	name         string
	before       string
	after        string
	unknown      string
	beforeSens   string
	afterSens    string
	replacePaths string
	noValues     bool
	want         []AttrChange
	wantReplaced []string
}

func runDiff(t *testing.T, tt diffCase) ([]AttrChange, []string) {
	t.Helper()
	c := PlanChange{
		Before:          decode(t, tt.before),
		After:           decode(t, tt.after),
		AfterUnknown:    decode(t, tt.unknown),
		BeforeSensitive: decode(t, tt.beforeSens),
		AfterSensitive:  decode(t, tt.afterSens),
	}
	if tt.replacePaths != "" {
		for _, p := range decode(t, tt.replacePaths).([]any) {
			c.ReplacePaths = append(c.ReplacePaths, p.([]any))
		}
	}
	return diffAttrs(c, !tt.noValues)
}

func TestDiffAttrs(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 47)
	tests := []diffCase{
		{
			name:   "no difference",
			before: `{"a":1,"b":{"c":"d"}}`,
			after:  `{"a":1,"b":{"c":"d"}}`,
		},
		{
			name:   "top-level scalar",
			before: `{"a":"x","b":1}`,
			after:  `{"a":"y","b":1}`,
			want:   []AttrChange{{Path: "a", Before: `"x"`, After: `"y"`}},
		},
		{
			name:   "nested map in key order",
			before: `{"tags":{"Name":"web","Env":"dev"}}`,
			after:  `{"tags":{"Name":"web-tier","Env":"prod"}}`,
			want: []AttrChange{
				{Path: "tags.Env", Before: `"dev"`, After: `"prod"`},
				{Path: "tags.Name", Before: `"web"`, After: `"web-tier"`},
			},
		},
		{
			name:   "key that is not an identifier",
			before: `{"tags":{"kubernetes.io/role":"elb"}}`,
			after:  `{"tags":{"kubernetes.io/role":"internal-elb"}}`,
			want:   []AttrChange{{Path: `tags["kubernetes.io/role"]`, Before: `"elb"`, After: `"internal-elb"`}},
		},
		{
			name:   "list element by index",
			before: `{"ingress":[{"cidr":["10.0.0.0/8"]}]}`,
			after:  `{"ingress":[{"cidr":["10.0.0.0/16"]}]}`,
			want:   []AttrChange{{Path: "ingress[0].cidr[0]", Before: `"10.0.0.0/8"`, After: `"10.0.0.0/16"`}},
		},
		{
			name:   "list grows with scalar",
			before: `{"ports":[80]}`,
			after:  `{"ports":[80,443]}`,
			want:   []AttrChange{{Path: "ports[1]", Before: "null", After: "443"}},
		},
		{
			name:   "list shrinks with object",
			before: `{"rules":[{"a":1},{"a":2}]}`,
			after:  `{"rules":[{"a":1}]}`,
			want:   []AttrChange{{Path: "rules[1]"}},
		},
		{
			name:   "null to value",
			before: `{"a":null}`,
			after:  `{"a":true}`,
			want:   []AttrChange{{Path: "a", Before: "null", After: "true"}},
		},
		{
			name:   "value to absent key",
			before: `{"a":"x"}`,
			after:  `{}`,
			want:   []AttrChange{{Path: "a", Before: `"x"`, After: "null"}},
		},
		{
			name:   "type change map to list",
			before: `{"a":{"b":1}}`,
			after:  `{"a":[1]}`,
			want:   []AttrChange{{Path: "a"}},
		},
		{
			name:   "scalar at the 48 byte limit is shown",
			before: `{"a":"x"}`,
			after:  `{"a":"` + long[:46] + `"}`,
			want:   []AttrChange{{Path: "a", Before: `"x"`, After: `"` + long[:46] + `"`}},
		},
		{
			name:   "scalar over 48 bytes shows the path only",
			before: `{"a":"short"}`,
			after:  `{"a":"` + long + `y"}`,
			want:   []AttrChange{{Path: "a"}},
		},
		{
			name:     "no values",
			before:   `{"a":"x"}`,
			after:    `{"a":"y"}`,
			noValues: true,
			want:     []AttrChange{{Path: "a"}},
		},
		{
			name:    "unknown with known before",
			before:  `{"id":"i-1","n":1}`,
			after:   `{"n":1}`,
			unknown: `{"id":true}`,
			want:    []AttrChange{{Path: "id", Before: `"i-1"`, Unknown: true}},
		},
		{
			name:    "unknown with null before",
			before:  `{"arn":null}`,
			after:   `{}`,
			unknown: `{"arn":true}`,
			want:    []AttrChange{{Path: "arn", Unknown: true}},
		},
		{
			name:     "unknown hides before when values are off",
			before:   `{"id":"i-1"}`,
			after:    `{}`,
			unknown:  `{"id":true}`,
			noValues: true,
			want:     []AttrChange{{Path: "id", Unknown: true}},
		},
		{
			name:    "unknown subtree stops recursion",
			before:  `{"block":{"a":1,"b":2}}`,
			after:   `{}`,
			unknown: `{"block":true}`,
			want:    []AttrChange{{Path: "block", Unknown: true}},
		},
		{
			name:    "unknown key only in after_unknown",
			before:  `{}`,
			after:   `{}`,
			unknown: `{"version":true}`,
			want:    []AttrChange{{Path: "version", Unknown: true}},
		},
		{
			name:    "unknown list element",
			before:  `{"l":[{"id":"a"}]}`,
			after:   `{"l":[{}]}`,
			unknown: `{"l":[{"id":true}]}`,
			want:    []AttrChange{{Path: "l[0].id", Before: `"a"`, Unknown: true}},
		},
		{
			name:       "sensitive changed",
			before:     `{"password":"hunter2"}`,
			after:      `{"password":"correct-horse"}`,
			beforeSens: `{"password":true}`,
			afterSens:  `{"password":true}`,
			want:       []AttrChange{{Path: "password", Sensitive: true}},
		},
		{
			name:       "sensitive unchanged is omitted",
			before:     `{"password":"same"}`,
			after:      `{"password":"same"}`,
			beforeSens: `{"password":true}`,
			afterSens:  `{"password":true}`,
		},
		{
			name:      "sensitive only after",
			before:    `{"token":"a"}`,
			after:     `{"token":"b"}`,
			afterSens: `{"token":true}`,
			want:      []AttrChange{{Path: "token", Sensitive: true}},
		},
		{
			name:       "sensitive and unknown",
			before:     `{"secret":"a"}`,
			after:      `{}`,
			unknown:    `{"secret":true}`,
			beforeSens: `{"secret":true}`,
			want:       []AttrChange{{Path: "secret", Sensitive: true, Unknown: true}},
		},
		{
			name:       "sensitive parent hides nested values",
			before:     `{"env":{"KEY":"old","OTHER":"x"}}`,
			after:      `{"env":{"KEY":"new","OTHER":"x"}}`,
			beforeSens: `{"env":true}`,
			afterSens:  `{"env":true}`,
			want:       []AttrChange{{Path: "env", Sensitive: true}},
		},
		{
			name:       "sensitive nested key",
			before:     `{"env":{"KEY":"old","LEVEL":"info"}}`,
			after:      `{"env":{"KEY":"new","LEVEL":"debug"}}`,
			beforeSens: `{"env":{"KEY":true}}`,
			afterSens:  `{"env":{"KEY":true}}`,
			want: []AttrChange{
				{Path: "env.KEY", Sensitive: true},
				{Path: "env.LEVEL", Before: `"info"`, After: `"debug"`},
			},
		},
		{
			name:       "sensitive root",
			before:     `{"a":"x"}`,
			after:      `{"a":"y"}`,
			beforeSens: `true`,
			want:       []AttrChange{{Path: "(resource)", Sensitive: true}},
		},
		{
			name:         "replace path marks and sorts first",
			before:       `{"a":1,"engine_version":"15"}`,
			after:        `{"a":2,"engine_version":"16"}`,
			replacePaths: `[["engine_version"]]`,
			want: []AttrChange{
				{Path: "engine_version", Before: `"15"`, After: `"16"`, ForcesReplacement: true},
				{Path: "a", Before: "1", After: "2"},
			},
			wantReplaced: []string{"engine_version"},
		},
		{
			name:         "replace path is a prefix of the attribute",
			before:       `{"nic":[{"subnet":"a"}]}`,
			after:        `{"nic":[{"subnet":"b"}]}`,
			replacePaths: `[["nic"]]`,
			want:         []AttrChange{{Path: "nic[0].subnet", Before: `"a"`, After: `"b"`, ForcesReplacement: true}},
			wantReplaced: []string{"nic"},
		},
		{
			name:         "attribute is a prefix of the replace path",
			before:       `{"nic":[{"subnet":"a"}]}`,
			after:        `{"nic":[]}`,
			replacePaths: `[["nic",0,"subnet"]]`,
			want:         []AttrChange{{Path: "nic[0]", ForcesReplacement: true}},
			wantReplaced: []string{"nic[0].subnet"},
		},
		{
			name:         "invalid replace path is ignored",
			before:       `{"a":1}`,
			after:        `{"a":2}`,
			replacePaths: `[[{"x":1}],["a",1.5]]`,
			want:         []AttrChange{{Path: "a", Before: "1", After: "2"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, replaced := runDiff(t, tt)
			if !equalAttrs(got, tt.want) {
				t.Errorf("attrs =\n%+v\nwant\n%+v", got, tt.want)
			}
			if strings.Join(replaced, "|") != strings.Join(tt.wantReplaced, "|") {
				t.Errorf("replacedBy = %q, want %q", replaced, tt.wantReplaced)
			}
		})
	}
}

func equalAttrs(a, b []AttrChange) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestDiffNeverRendersSensitiveValues(t *testing.T) {
	t.Parallel()
	secrets := []string{"s3cr3t-before", "s3cr3t-after", "nested-old", "nested-new", "list-old", "list-new"}
	c := PlanChange{
		Before:          decode(t, `{"a":"s3cr3t-before","m":{"k":"nested-old"},"l":["list-old"]}`),
		After:           decode(t, `{"a":"s3cr3t-after","m":{"k":"nested-new"},"l":["list-new"]}`),
		BeforeSensitive: decode(t, `{"a":true,"m":{"k":true},"l":[true]}`),
		AfterSensitive:  decode(t, `{"a":true,"m":{"k":true},"l":[true]}`),
	}
	attrs, _ := diffAttrs(c, true)
	if len(attrs) != 3 {
		t.Fatalf("attrs = %+v, want 3 sensitive changes", attrs)
	}
	out, err := json.Marshal(attrs)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range secrets {
		if strings.Contains(string(out), s) {
			t.Errorf("attrs leak %q: %s", s, out)
		}
	}
}

func TestPathString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		p    path
		want string
	}{
		{nil, "(resource)"},
		{path{{key: "a"}}, "a"},
		{path{{key: "a"}, {key: "b_c"}}, "a.b_c"},
		{path{{key: "a"}, {index: 3, isIndex: true}, {key: "b"}}, "a[3].b"},
		{path{{key: "tags"}, {key: "a.b/c"}}, `tags["a.b/c"]`},
		{path{{key: "1abc"}}, `["1abc"]`},
		{path{{key: ""}}, `[""]`},
		{path{{key: "with-dash"}}, "with-dash"},
	}
	for _, tt := range tests {
		if got := tt.p.String(); got != tt.want {
			t.Errorf("path%+v.String() = %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestScalar(t *testing.T) {
	t.Parallel()
	tests := []struct {
		v    any
		want string
		ok   bool
	}{
		{nil, "null", true},
		{true, "true", true},
		{false, "false", true},
		{"a\"b", `"a\"b"`, true},
		{json.Number("42"), "42", true},
		{1.5, "1.5", true},
		{map[string]any{}, "", false},
		{[]any{}, "", false},
		{strings.Repeat("x", 46), `"` + strings.Repeat("x", 46) + `"`, true},
		{strings.Repeat("x", 47), "", false},
	}
	for _, tt := range tests {
		got, ok := scalar(tt.v)
		if got != tt.want || ok != tt.ok {
			t.Errorf("scalar(%v) = %q, %v; want %q, %v", tt.v, got, ok, tt.want, tt.ok)
		}
	}
}

func TestToPath(t *testing.T) {
	t.Parallel()
	p, ok := toPath([]any{"a", json.Number("2"), 3.0, "b"})
	if !ok || p.String() != "a[2][3].b" {
		t.Errorf("toPath() = %v, %v", p, ok)
	}
	if _, ok := toPath([]any{true}); ok {
		t.Error("toPath(bool) succeeded")
	}
	if _, ok := toPath([]any{json.Number("1.5")}); ok {
		t.Error("toPath(1.5) succeeded")
	}
}

func TestChildOutOfRange(t *testing.T) {
	t.Parallel()
	list := []any{"a"}
	for _, s := range []step{{index: -1, isIndex: true}, {index: 1, isIndex: true}} {
		if got := child(list, s); got != nil {
			t.Errorf("child(%v) = %v, want nil", s, got)
		}
	}
	if got := child("scalar", step{key: "a"}); got != nil {
		t.Errorf("child(scalar) = %v, want nil", got)
	}
}

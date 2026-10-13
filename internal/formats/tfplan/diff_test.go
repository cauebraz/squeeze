package tfplan

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var ignoreAt = cmpopts.IgnoreUnexported(AttrChange{})

func sv(v any) *Scalar {
	return &Scalar{Value: v}
}

func num(n string) *Scalar {
	return &Scalar{Value: json.Number(n)}
}

type diffCase struct {
	before, after, unknown, beforeSens, afterSens, replacePaths string
	noValues                                                    bool
	want                                                        []AttrChange
	wantReplaced                                                []string
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
	long := strings.Repeat("x", 60)
	tests := map[string]diffCase{
		"no difference": {
			before: `{"a":1,"b":{"c":"d"}}`, after: `{"a":1,"b":{"c":"d"}}`,
		},
		"top-level scalar": {
			before: `{"a":"x","b":1}`, after: `{"a":"y","b":1}`,
			want: []AttrChange{{Path: "a", Before: sv("x"), After: sv("y")}},
		},
		"nested map in key order": {
			before: `{"tags":{"Name":"web","Env":"dev"}}`, after: `{"tags":{"Name":"web-tier","Env":"prod"}}`,
			want: []AttrChange{
				{Path: "tags.Env", Before: sv("dev"), After: sv("prod")},
				{Path: "tags.Name", Before: sv("web"), After: sv("web-tier")},
			},
		},
		"key that is not an identifier": {
			before: `{"tags":{"kubernetes.io/role":"elb"}}`, after: `{"tags":{"kubernetes.io/role":"internal-elb"}}`,
			want: []AttrChange{{Path: `tags["kubernetes.io/role"]`, Before: sv("elb"), After: sv("internal-elb")}},
		},
		"map key added": {
			before: `{"tags":{"Team":"platform"}}`, after: `{"tags":{"Team":"platform","ManagedBy":"terraform"}}`,
			want: []AttrChange{{Path: "tags.ManagedBy", Kind: Added, After: sv("terraform")}},
		},
		"map key removed": {
			before: `{"tags":{"Owner":"someone"}}`, after: `{"tags":{}}`,
			want: []AttrChange{{Path: "tags.Owner", Kind: Removed, Before: sv("someone")}},
		},
		"explicit null is a modification": {
			before: `{"a":null}`, after: `{"a":true}`,
			want: []AttrChange{{Path: "a", Before: sv(nil), After: sv(true)}},
		},
		"value to explicit null": {
			before: `{"a":"x"}`, after: `{"a":null}`,
			want: []AttrChange{{Path: "a", Before: sv("x"), After: sv(nil)}},
		},
		"list element by index": {
			before: `{"ingress":[{"cidr":["10.0.0.0/8"]}]}`, after: `{"ingress":[{"cidr":["10.0.0.0/16"]}]}`,
			want: []AttrChange{{Path: "ingress[0].cidr[0]", Before: sv("10.0.0.0/8"), After: sv("10.0.0.0/16")}},
		},
		"list grows": {
			before: `{"ports":[80]}`, after: `{"ports":[80,443]}`,
			want: []AttrChange{{Path: "ports[1]", Kind: Added, After: num("443")}},
		},
		"list shrinks with object": {
			before: `{"rules":[{"a":1},{"a":2}]}`, after: `{"rules":[{"a":1}]}`,
			want: []AttrChange{{Path: "rules[1]", Kind: Removed}},
		},
		"type change map to list": {
			before: `{"a":{"b":1}}`, after: `{"a":[1]}`,
			want: []AttrChange{{Path: "a"}},
		},
		"long values stay in the model": {
			before: `{"a":"short"}`, after: `{"a":"` + long + `"}`,
			want: []AttrChange{{Path: "a", Before: sv("short"), After: sv(long)}},
		},
		"no values": {
			before: `{"a":"x","t":{}}`, after: `{"a":"y","t":{"k":"v"}}`, noValues: true,
			want: []AttrChange{{Path: "a"}, {Path: "t.k", Kind: Added}},
		},
		"unknown with known before": {
			before: `{"id":"i-1","n":1}`, after: `{"n":1}`, unknown: `{"id":true}`,
			want: []AttrChange{{Path: "id", Before: sv("i-1"), Unknown: true}},
		},
		"unknown with null before": {
			before: `{"arn":null}`, after: `{}`, unknown: `{"arn":true}`,
			want: []AttrChange{{Path: "arn", Unknown: true}},
		},
		"unknown hides before when values are off": {
			before: `{"id":"i-1"}`, after: `{}`, unknown: `{"id":true}`, noValues: true,
			want: []AttrChange{{Path: "id", Unknown: true}},
		},
		"unknown subtree stops recursion": {
			before: `{"block":{"a":1,"b":2}}`, after: `{}`, unknown: `{"block":true}`,
			want: []AttrChange{{Path: "block", Unknown: true}},
		},
		"unknown key only in after_unknown is added": {
			before: `{}`, after: `{}`, unknown: `{"version":true}`,
			want: []AttrChange{{Path: "version", Kind: Added, Unknown: true}},
		},
		"unknown list element": {
			before: `{"l":[{"id":"a"}]}`, after: `{"l":[{}]}`, unknown: `{"l":[{"id":true}]}`,
			want: []AttrChange{{Path: "l[0].id", Before: sv("a"), Unknown: true}},
		},
		"after_unknown false entries are ignored": {
			before: `{"a":1}`, after: `{"a":1}`, unknown: `{"a":false,"b":false,"c":{}}`,
		},
		"sensitive changed": {
			before: `{"password":"hunter2"}`, after: `{"password":"correct-horse"}`,
			beforeSens: `{"password":true}`, afterSens: `{"password":true}`,
			want: []AttrChange{{Path: "password", Sensitive: true}},
		},
		"sensitive unchanged is omitted": {
			before: `{"password":"same"}`, after: `{"password":"same"}`,
			beforeSens: `{"password":true}`, afterSens: `{"password":true}`,
		},
		"sensitive only after": {
			before: `{"token":"a"}`, after: `{"token":"b"}`, afterSens: `{"token":true}`,
			want: []AttrChange{{Path: "token", Sensitive: true}},
		},
		"sensitive added": {
			before: `{"env":{}}`, after: `{"env":{"KEY":"v"}}`, afterSens: `{"env":{"KEY":true}}`,
			want: []AttrChange{{Path: "env.KEY", Kind: Added, Sensitive: true}},
		},
		"sensitive and unknown": {
			before: `{"secret":"a"}`, after: `{}`, unknown: `{"secret":true}`, beforeSens: `{"secret":true}`,
			want: []AttrChange{{Path: "secret", Sensitive: true, Unknown: true}},
		},
		"sensitive parent hides nested values": {
			before: `{"env":{"KEY":"old","OTHER":"x"}}`, after: `{"env":{"KEY":"new","OTHER":"x"}}`,
			beforeSens: `{"env":true}`, afterSens: `{"env":true}`,
			want: []AttrChange{{Path: "env", Sensitive: true}},
		},
		"sensitive nested key": {
			before: `{"env":{"KEY":"old","LEVEL":"info"}}`, after: `{"env":{"KEY":"new","LEVEL":"debug"}}`,
			beforeSens: `{"env":{"KEY":true}}`, afterSens: `{"env":{"KEY":true}}`,
			want: []AttrChange{
				{Path: "env.KEY", Sensitive: true},
				{Path: "env.LEVEL", Before: sv("info"), After: sv("debug")},
			},
		},
		"sensitive root": {
			before: `{"a":"x"}`, after: `{"a":"y"}`, beforeSens: `true`,
			want: []AttrChange{{Path: "(resource)", Sensitive: true}},
		},
		"replace path marks and sorts first": {
			before: `{"a":1,"engine_version":"15"}`, after: `{"a":2,"engine_version":"16"}`,
			replacePaths: `[["engine_version"]]`,
			want: []AttrChange{
				{Path: "engine_version", Before: sv("15"), After: sv("16"), ForcesReplacement: true},
				{Path: "a", Before: num("1"), After: num("2")},
			},
			wantReplaced: []string{"engine_version"},
		},
		"replace path is a prefix of the attribute": {
			before: `{"nic":[{"subnet":"a"}]}`, after: `{"nic":[{"subnet":"b"}]}`, replacePaths: `[["nic"]]`,
			want:         []AttrChange{{Path: "nic[0].subnet", Before: sv("a"), After: sv("b"), ForcesReplacement: true}},
			wantReplaced: []string{"nic"},
		},
		"attribute is a prefix of the replace path": {
			before: `{"nic":[{"subnet":"a"}]}`, after: `{"nic":[]}`, replacePaths: `[["nic",0,"subnet"]]`,
			want:         []AttrChange{{Path: "nic[0]", Kind: Removed, ForcesReplacement: true}},
			wantReplaced: []string{"nic[0].subnet"},
		},
		"invalid replace path is ignored": {
			before: `{"a":1}`, after: `{"a":2}`, replacePaths: `[[{"x":1}],["a",1.5]]`,
			want: []AttrChange{{Path: "a", Before: num("1"), After: num("2")}},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, replaced := runDiff(t, tt)
			if diff := cmp.Diff(tt.want, got, ignoreAt, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("attrs (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(tt.wantReplaced, replaced, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("replacedBy (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDiffAttrsCarryTheirPath(t *testing.T) {
	t.Parallel()
	got, _ := runDiff(t, diffCase{before: `{"m":{"k":[1]}}`, after: `{"m":{"k":[2]}}`})
	want := path{{key: "m"}, {key: "k"}, {index: 0, isIndex: true}}
	if len(got) != 1 || !got[0].at.hasPrefix(want) || len(got[0].at) != len(want) {
		t.Errorf("at = %v, want %v", got[0].at, want)
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

func TestPathOverlaps(t *testing.T) {
	t.Parallel()
	a := path{{key: "a"}}
	ab := path{{key: "a"}, {key: "b"}}
	c := path{{key: "c"}}
	if !a.overlaps(ab) || !ab.overlaps(a) || a.overlaps(c) || !a.overlaps(nil) {
		t.Error("overlaps() returned a wrong answer")
	}
}

func TestToPath(t *testing.T) {
	t.Parallel()
	p, ok := toPath([]any{"a", json.Number("2"), 3.0, "b"})
	if !ok || p.String() != "a[2][3].b" {
		t.Errorf("toPath() = %v, %v", p, ok)
	}
	for _, bad := range [][]any{{true}, {json.Number("1.5")}, {1.5}} {
		if _, ok := toPath(bad); ok {
			t.Errorf("toPath(%v) succeeded", bad)
		}
	}
}

func TestLookup(t *testing.T) {
	t.Parallel()
	list := []any{"a"}
	for _, s := range []step{{index: -1, isIndex: true}, {index: 1, isIndex: true}} {
		if v, ok := lookup(list, s); ok || v != nil {
			t.Errorf("lookup(%v) = %v, %v", s, v, ok)
		}
	}
	if _, ok := lookup("scalar", step{key: "a"}); ok {
		t.Error("lookup(scalar) found a key")
	}
	if v, ok := lookup(map[string]any{"k": nil}, step{key: "k"}); !ok || v != nil {
		t.Errorf("lookup(explicit null) = %v, %v", v, ok)
	}
}

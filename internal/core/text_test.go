package core

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNoticeName(t *testing.T) {
	tests := map[string]string{
		"":                  "",
		"plain":             "plain",
		"weird (x)":         "weird [x]",
		"a(b)c)(":           "a[b]c][",
		"already [bracket]": "already [bracket]",
	}
	for in, want := range tests {
		if diff := cmp.Diff(want, NoticeName(in)); diff != "" {
			t.Errorf("NoticeName(%q) (-want +got):\n%s", in, diff)
		}
	}
}

func TestDedent(t *testing.T) {
	tests := map[string]struct {
		in, want []string
	}{
		"empty":         {in: nil, want: nil},
		"no indent":     {in: []string{"a", "  b"}, want: []string{"a", "  b"}},
		"common indent": {in: []string{"    a", "      b", "    c"}, want: []string{"a", "  b", "c"}},
		"tabs kept":     {in: []string{"  \ta", "  b"}, want: []string{"\ta", "b"}},
		"blank line":    {in: []string{"  a", ""}, want: []string{"  a", ""}},
	}
	for name, tc := range tests {
		if diff := cmp.Diff(tc.want, Dedent(tc.in)); diff != "" {
			t.Errorf("%s: Dedent() (-want +got):\n%s", name, diff)
		}
	}
}

func TestSplitLines(t *testing.T) {
	tests := map[string][]string{
		"":             {""},
		"a":            {"a"},
		"a\n":          {"a", ""},
		"a\r\nb\r\r\n": {"a", "b", ""},
		"a\rb\n\nc":    {"a\rb", "", "c"},
	}
	for in, want := range tests {
		if diff := cmp.Diff(want, slices.Collect(SplitLines(in))); diff != "" {
			t.Errorf("SplitLines(%q) (-want +got):\n%s", in, diff)
		}
	}
	for l := range SplitLines("a\nb\nc") {
		if l != "a" {
			t.Errorf("first line = %q", l)
		}
		break
	}
}

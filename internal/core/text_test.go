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

func TestPlural(t *testing.T) {
	t.Parallel()
	if Plural(1, "line", "lines") != "line" || Plural(0, "line", "lines") != "lines" || Plural(2, "line", "lines") != "lines" {
		t.Error("Plural() picks the wrong form")
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"abc", 0, "abc"},
		{"abc", 5, "abc"},
		{"abcdef", 3, "abc"},
		{"aéb", 2, "a"},
		{"éb", 1, "é"},
		{"漢字", 3, "漢"},
	}
	for _, tt := range tests {
		if got := Truncate(tt.s, tt.n); got != tt.want {
			t.Errorf("Truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}

func TestToolLabel(t *testing.T) {
	t.Parallel()
	if got := ToolLabel("trivy", "0.75.0"); got != "trivy 0.75.0" {
		t.Errorf("ToolLabel(trivy, 0.75.0) = %q", got)
	}
	if got := ToolLabel("trivy", ""); got != "trivy" {
		t.Errorf("ToolLabel(trivy, \"\") = %q", got)
	}
}

func TestEllipsis(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		in   string
		n    int
		want string
	}{
		{"abcdef", 6, "abcdef"},
		{"abcdefg", 6, "abc..."},
		{"ab\u00e9def", 6, "ab..."},
		{"", 6, ""},
		{"abcdef", 3, "abc"},
		{"abcdef", 2, "ab"},
		{"abcdef", 0, "abcdef"},
	} {
		if got := Ellipsis(tt.in, tt.n); got != tt.want {
			t.Errorf("Ellipsis(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
		}
	}
}

func TestSeconds(t *testing.T) {
	t.Parallel()
	if Seconds(1.5) != "1.5s" || Seconds(2) != "2s" || Seconds(0.001) != "0.001s" {
		t.Error("Seconds() format")
	}
}

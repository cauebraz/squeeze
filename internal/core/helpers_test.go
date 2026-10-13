package core

import (
	"flag"
	"io"
	"strings"
	"testing"
)

func TestCountFlag(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		args []string
		msg  string
	}{{[]string{"-n", "x"}, "not an integer"}, {[]string{"-n", "-1"}, "must be 0 or more"}} {
		n := 7
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		CountFlag(fs, &n, "n", "things listed, 0 for all")
		if err := fs.Parse(tt.args); err == nil || !strings.Contains(err.Error(), tt.msg) {
			t.Errorf("Parse(%v) error = %v, want %q", tt.args, err, tt.msg)
		}
	}
	n := 7
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	CountFlag(fs, &n, "n", "things listed, 0 for all")
	if err := fs.Parse([]string{"-n", "3"}); err != nil || n != 3 {
		t.Errorf("Parse(-n 3): n = %d, err = %v", n, err)
	}
	if usage := fs.Lookup("n").Usage; usage != "`n` things listed, 0 for all (default 7)" {
		t.Errorf("usage = %q", usage)
	}
}

func TestPlural(t *testing.T) {
	t.Parallel()
	if Plural(1, "line", "lines") != "line" || Plural(0, "line", "lines") != "lines" || Plural(2, "line", "lines") != "lines" {
		t.Error("Plural() picks the wrong form")
	}
}

func TestLimitAndLimitTail(t *testing.T) {
	t.Parallel()
	items := []int{1, 2, 3, 4, 5}
	for _, tt := range []struct{ n, kept, omitted int }{{0, 5, 0}, {5, 5, 0}, {7, 5, 0}, {2, 2, 3}} {
		kept, omitted := Limit(items, tt.n)
		if len(kept) != tt.kept || omitted != tt.omitted {
			t.Errorf("Limit(%d) = %d, %d; want %d, %d", tt.n, len(kept), omitted, tt.kept, tt.omitted)
		}
		tail, omitted := LimitTail(items, tt.n)
		if len(tail) != tt.kept || omitted != tt.omitted || tail[len(tail)-1] != 5 {
			t.Errorf("LimitTail(%d) = %v, %d", tt.n, tail, omitted)
		}
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

func TestSeconds(t *testing.T) {
	t.Parallel()
	if Seconds(1.5) != "1.5s" || Seconds(2) != "2s" || Seconds(0.001) != "0.001s" {
		t.Error("Seconds() format")
	}
}

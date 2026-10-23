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

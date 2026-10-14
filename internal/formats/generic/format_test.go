package generic

import (
	"bytes"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
	"github.com/google/go-cmp/cmp"
)

const fixtureDir = "testdata"

func TestContract(t *testing.T) {
	t.Parallel()
	formattest.Contract(t, Format{}, fixtureDir)
}

func TestRealInputs(t *testing.T) {
	t.Parallel()
	formattest.Real(t, Format{}, "testdata/real")
}

func TestDetect(t *testing.T) {
	t.Parallel()
	full := bytes.Repeat([]byte("a"), core.HeadSize-1)
	tests := map[string]struct {
		input []byte
		want  bool
	}{
		"text":                 {[]byte("collected 3 items\n"), true},
		"escapes":              {[]byte("\x1b[0m\n"), true},
		"blank":                {[]byte(" \n\t"), false},
		"nul byte":             {[]byte("ELF\x00\x01"), false},
		"invalid utf-8":        {[]byte("caf\xe9 au lait"), false},
		"rune cut by the head": {append(append([]byte(nil), full...), 0xc3), true},
		"lone continuation":    {[]byte("abc\x80"), false},
		"head of whitespace":   {bytes.Repeat([]byte("\n"), core.HeadSize), true},
	}
	for name, tt := range tests {
		if got := (Format{}).Detect(tt.input); got != tt.want {
			t.Errorf("%s: Detect() = %v, want %v", name, got, tt.want)
		}
	}
}

func lines(s ...string) []byte {
	return []byte(strings.Join(s, "\n") + "\n")
}

func TestNegatedFlagWords(t *testing.T) {
	t.Parallel()
	tests := map[string][2]bool{
		"error: layer missing":                            {true, false},
		"1 warning, 0 errors":                             {false, true},
		"0 warnings suppressed":                           {false, false},
		"[001/400] ok (3 symbols, 0 warnings suppressed)": {false, false},
		"no errors found":                                 {false, false},
		"build finished without warnings":                 {false, false},
		"errors: 0, warnings: 2":                          {false, true},
		"warnings=0":                                      {false, false},
		"non-fatal":                                       {false, false},
		"Fatal error":                                     {true, false},
		"12 failed, 3 passed":                             {true, false},
		"zero failures":                                   {false, false},
	}
	for line, want := range tests {
		if got := [2]bool{isError(line), isWarning(line)}; got != want {
			t.Errorf("%q: error, warning = %v, want %v", line, got, want)
		}
	}
}

func TestText(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input []byte
		o     Options
		want  string
	}{
		"carriage return keeps the last frame": {
			input: []byte("10%\r50%\r100%\ndone\r\n"),
			o:     DefaultOptions(),
			want:  "100%\ndone\n",
		},
		"trailing carriage return keeps the last text": {
			input: []byte("loading\r\r\nnext\n"),
			o:     DefaultOptions(),
			want:  "loading\nnext\n",
		},
		"frame of escape codes keeps the text before it": {
			input: []byte("Downloading 100%\r\x1b[0m\nDone\r\x1b[K\n"),
			o:     DefaultOptions(),
			want:  "Downloading 100%\nDone\n",
		},
		"plurals of flag words do not fold": {
			input: lines("x 1 panics", "x 2 panics", "x 3 panics", "y 1 tracebacks", "y 2 tracebacks", "y 3 tracebacks"),
			o:     DefaultOptions(),
			want:  "x 1 panics\nx 2 panics\nx 3 panics\ny 1 tracebacks\ny 2 tracebacks\ny 3 tracebacks\n",
		},
		"fold notices do not count toward the limit": {
			input: lines("a", "a", "a", "b", "b", "b", "c", "d", "e", "f", "g", "g", "g"),
			o:     Options{MaxLines: 4},
			want:  "a\n... 2 more identical lines (--no-fold)\n... 6 lines cut (--max-lines 0)\nf\ng\n... 2 more identical lines (--no-fold)\n",
		},
		"cut never leaves a fold notice without its line": {
			input: lines("a", "b", "c", "d", "e", "same", "same", "same", "same", "z"),
			o:     Options{MaxLines: 4},
			want:  "a\n... 4 lines cut (--max-lines 0)\nsame\n... 3 more identical lines (--no-fold)\nz\n",
		},
		"one byte keeps the first character": {
			input: lines("\u00e9\u6f22"),
			o:     Options{MaxLineBytes: 1},
			want:  "\u00e9 ... 3 more bytes (--max-line-bytes 0)\n",
		},
		"blank runs collapse": {
			input: lines("", "", "a", "", "", "", "b", "", ""),
			o:     DefaultOptions(),
			want:  "a\n\nb\n",
		},
		"two identical lines stay": {
			input: lines("x", "x"),
			o:     DefaultOptions(),
			want:  "x\nx\n",
		},
		"identical run folds": {
			input: lines("x", "x", "x", "x"),
			o:     DefaultOptions(),
			want:  "x\n... 3 more identical lines (--no-fold)\n",
		},
		"similar run keeps first and last": {
			input: lines("got 1 of 9", "got 2 of 9", "got 3 of 9", "got 9 of 9"),
			o:     DefaultOptions(),
			want:  "got 1 of 9\n... 2 similar lines (--no-fold)\ngot 9 of 9\n",
		},
		"similar error lines do not fold": {
			input: lines("error at line 1", "error at line 2", "error at line 3"),
			o:     DefaultOptions(),
			want:  "error at line 1\nerror at line 2\nerror at line 3\n",
		},
		"identical error lines fold": {
			input: lines("error: x", "error: x", "error: x"),
			o:     DefaultOptions(),
			want:  "error: x\n... 2 more identical lines (--no-fold)\n",
		},
		"no fold": {
			input: lines("x", "x", "x"),
			o:     Options{NoFold: true},
			want:  "x\nx\nx\n",
		},
		"cut keeps head, tail and flagged lines": {
			input: lines("a1", "b", "c", "d", "Error: e", "f", "g", "h", "i", "j"),
			o:     Options{MaxLines: 4},
			want:  "a1\n... 3 lines cut (--max-lines 0)\nError: e\n... 3 lines cut (--max-lines 0)\ni\nj\n",
		},
		"cut counts flagged lines over the budget": {
			input: lines("a", "warning 1", "warning 2", "warning 3", "b", "c"),
			o:     Options{MaxLines: 4},
			want:  "a\nwarning 1\n... 2 lines cut, 2 with error or warning words (--max-lines 0)\nb\nc\n",
		},
		"lines with zero warnings fold": {
			input: lines("[1/4] ok (0 warnings)", "[2/4] ok (0 warnings)", "[3/4] ok (0 warnings)", "error: x", "[4/4] ok (0 warnings)"),
			o:     DefaultOptions(),
			want:  "[1/4] ok (0 warnings)\n... 1 similar line (--no-fold)\n[3/4] ok (0 warnings)\nerror: x\n[4/4] ok (0 warnings)\n",
		},
		"cut fold marker counts its lines": {
			input: lines("a", "b", "x", "x", "x", "x", "c", "d"),
			o:     Options{MaxLines: 4},
			want:  "a\n... 5 lines cut (--max-lines 0)\nc\nd\n",
		},
		"long line": {
			input: lines(strings.Repeat("ab", 10), "ok"),
			o:     Options{MaxLineBytes: 5},
			want:  "ababa ... 15 more bytes (--max-line-bytes 0)\nok\n",
		},
		"long line cut on a rune boundary": {
			input: lines("aaaaéz"),
			o:     Options{MaxLineBytes: 5},
			want:  "aaaa ... 3 more bytes (--max-line-bytes 0)\n",
		},
		"indentation and tabs": {
			input: lines("  two", "\tTab"),
			o:     DefaultOptions(),
			want:  "  two\n    Tab\n",
		},
		"nothing printable": {
			input: []byte("\x1b[0m\n\x07\n"),
			o:     DefaultOptions(),
			want:  "(no printable text)\n",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, err := Squeeze(tt.input, tt.o)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, r.Text); diff != "" {
				t.Errorf("text (-want +got):\n%s", diff)
			}
			checkAccounting(t, r.Data.(Report))
		})
	}
}

func TestReport(t *testing.T) {
	t.Parallel()
	r, _ := Build(lines("", "a", "a", "a", "b", "", "", strings.Repeat("z", 9), "Error x", "c", "d"), Options{MaxLines: 5, MaxLineBytes: 4})
	want := Report{Tool: "text", Lines: 11, Printed: 4, BlankDropped: 2, Folded: 2, Cut: 3, LongLines: 1, ErrorLines: 1}
	if diff := cmp.Diff(want, r); diff != "" {
		t.Errorf("report (-want +got):\n%s", diff)
	}
}

func checkAccounting(t *testing.T, r Report) {
	t.Helper()
	if got := r.Printed + r.BlankDropped + r.Folded + r.Cut; got != r.Lines {
		t.Errorf("printed %d + blank %d + folded %d + cut %d = %d, want %d input lines", r.Printed, r.BlankDropped, r.Folded, r.Cut, got, r.Lines)
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte("a\r\rb\n\x1b]0;t\x07x\n\n\nerror 1\nerror 1\nerror 1\n\x1b[1;2"))
	f.Fuzz(func(t *testing.T, input []byte) {
		for _, o := range []Options{DefaultOptions(), {MaxLines: 3, MaxLineBytes: 2}, {NoFold: true, MaxLines: 1}} {
			res, err := Squeeze(input, o)
			if err != nil {
				t.Fatal(err)
			}
			formattest.CheckText(t, res.Text)
			if strings.ContainsRune(res.Text, 0x1b) {
				t.Fatalf("escape byte in output: %q", res.Text)
			}
			r := res.Data.(Report)
			checkAccounting(t, r)
			content := r.Printed
			if o.MaxLines > 0 && content > o.MaxLines {
				t.Fatalf("%d lines printed over the limit %d", content, o.MaxLines)
			}
			if r.Cut > 0 && !strings.Contains(res.Text, "(--max-lines 0)") {
				t.Fatalf("%d lines cut without a notice:\n%s", r.Cut, res.Text)
			}
			if r.Folded > 0 && !strings.Contains(res.Text, "(--no-fold)") {
				t.Fatalf("%d lines folded without a notice:\n%s", r.Folded, res.Text)
			}
			if _, err := core.Envelope("generic", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

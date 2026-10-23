package generic

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
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

func TestText(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input []byte
		o     options
		want  string
	}{
		"carriage return keeps the last frame": {
			input: []byte("10%\r50%\r100%\ndone\r\n"),
			o:     defaultOptions(),
			want:  "100%\ndone\n",
		},
		"trailing carriage return keeps the last text": {
			input: []byte("loading\r\r\nnext\n"),
			o:     defaultOptions(),
			want:  "loading\nnext\n",
		},
		"frame of escape codes keeps the text before it": {
			input: []byte("Downloading 100%\r\x1b[0m\nDone\r\x1b[K\n"),
			o:     defaultOptions(),
			want:  "Downloading 100%\nDone\n",
		},
		"plurals of flag words do not fold": {
			input: lines("x 1 panics", "x 2 panics", "x 3 panics", "y 1 tracebacks", "y 2 tracebacks", "y 3 tracebacks"),
			o:     defaultOptions(),
			want:  "x 1 panics\nx 2 panics\nx 3 panics\ny 1 tracebacks\ny 2 tracebacks\ny 3 tracebacks\n",
		},
		"fold notices do not count toward the limit": {
			input: lines("a", "a", "a", "b", "b", "b", "c", "d", "e", "f", "g", "g", "g"),
			o:     options{MaxLines: 4},
			want:  "a\n... 2 more identical lines (--no-fold)\n... 6 lines cut (--max-lines 0)\nf\ng\n... 2 more identical lines (--no-fold)\n",
		},
		"similar warning lines fold and the error line survives the cut": {
			input: lines("w 1 warnings", "w 2 warnings", "w 3 warnings", "w 4 warnings", "x", "error: boom", "y", "w 5 warnings", "w 6 warnings", "w 7 warnings", "z"),
			o:     options{MaxLines: 4},
			want:  "w 1 warnings\n... 2 similar lines, 2 with warning words (--no-fold)\nw 4 warnings\n... 1 line cut (--max-lines 0)\nerror: boom\n... 4 lines cut, 3 with error or warning words (--max-lines 0)\nz\n",
		},
		"error lines fold only when identical": {
			input: lines("error 1", "error 2", "error 3", "error 3", "error 3"),
			o:     defaultOptions(),
			want:  "error 1\nerror 2\nerror 3\n... 2 more identical lines (--no-fold)\n",
		},
		"cut never leaves a fold notice without its line": {
			input: lines("a", "b", "c", "d", "e", "same", "same", "same", "same", "z"),
			o:     options{MaxLines: 4},
			want:  "a\n... 4 lines cut (--max-lines 0)\nsame\n... 3 more identical lines (--no-fold)\nz\n",
		},
		"one byte keeps the first character": {
			input: lines("\u00e9\u6f22"),
			o:     options{MaxLineBytes: 1},
			want:  "\u00e9 ... 3 more bytes (--max-line-bytes 0)\n",
		},
		"blank runs collapse": {
			input: lines("", "", "a", "", "", "", "b", "", ""),
			o:     defaultOptions(),
			want:  "a\n\nb\n",
		},
		"two identical lines stay": {
			input: lines("x", "x"),
			o:     defaultOptions(),
			want:  "x\nx\n",
		},
		"identical run folds": {
			input: lines("x", "x", "x", "x"),
			o:     defaultOptions(),
			want:  "x\n... 3 more identical lines (--no-fold)\n",
		},
		"similar run keeps first and last": {
			input: lines("got 1 of 9", "got 2 of 9", "got 3 of 9", "got 9 of 9"),
			o:     defaultOptions(),
			want:  "got 1 of 9\n... 2 similar lines (--no-fold)\ngot 9 of 9\n",
		},
		"similar error lines do not fold": {
			input: lines("error at line 1", "error at line 2", "error at line 3"),
			o:     defaultOptions(),
			want:  "error at line 1\nerror at line 2\nerror at line 3\n",
		},
		"identical error lines fold": {
			input: lines("error: x", "error: x", "error: x"),
			o:     defaultOptions(),
			want:  "error: x\n... 2 more identical lines (--no-fold)\n",
		},
		"no fold": {
			input: lines("x", "x", "x"),
			o:     options{NoFold: true},
			want:  "x\nx\nx\n",
		},
		"cut keeps head, tail and flagged lines": {
			input: lines("a1", "b", "c", "d", "Error: e", "f", "g", "h", "i", "j"),
			o:     options{MaxLines: 4},
			want:  "a1\n... 3 lines cut (--max-lines 0)\nError: e\n... 3 lines cut (--max-lines 0)\ni\nj\n",
		},
		"cut counts flagged lines over the budget": {
			input: lines("a", "warning x", "warning y", "warning z", "b", "c"),
			o:     options{MaxLines: 4},
			want:  "a\nwarning x\n... 2 lines cut, 2 with error or warning words (--max-lines 0)\nb\nc\n",
		},
		"lines with zero warnings fold": {
			input: lines("[1/4] ok (0 warnings)", "[2/4] ok (0 warnings)", "[3/4] ok (0 warnings)", "error: x", "[4/4] ok (0 warnings)"),
			o:     defaultOptions(),
			want:  "[1/4] ok (0 warnings)\n... 1 similar line (--no-fold)\n[3/4] ok (0 warnings)\nerror: x\n[4/4] ok (0 warnings)\n",
		},
		"cut fold marker counts its lines": {
			input: lines("a", "b", "x", "x", "x", "x", "c", "d"),
			o:     options{MaxLines: 4},
			want:  "a\n... 5 lines cut (--max-lines 0)\nc\nd\n",
		},
		"long line": {
			input: lines(strings.Repeat("ab", 10), "ok"),
			o:     options{MaxLineBytes: 5},
			want:  "ababa ... 15 more bytes (--max-line-bytes 0)\nok\n",
		},
		"long line cut on a rune boundary": {
			input: lines("aaaaéz"),
			o:     options{MaxLineBytes: 5},
			want:  "aaaa ... 3 more bytes (--max-line-bytes 0)\n",
		},
		"indentation and tabs": {
			input: lines("  two", "\tTab", "id\tname\tcpu", "7\tsvc-a\t42", "\t\tat main.go:3", "x\t"),
			o:     defaultOptions(),
			want:  "  two\n Tab\nid name cpu\n7 svc-a 42\n  at main.go:3\nx\n",
		},
		"an exception line and the line before it survive the cut": {
			input: lines("start", "a", "b", "c", "d", "e", "    raise ValueError(\"bad\")", "ValueError: bad", "f", "g", "h", "i", "end 1", "end 2"),
			o:     options{MaxLines: 6},
			want:  "start\n... 5 lines cut (--max-lines 0)\n    raise ValueError(\"bad\")\nValueError: bad\n... 3 lines cut (--max-lines 0)\ni\nend 1\nend 2\n",
		},
		"nothing printable": {
			input: []byte("\x1b[0m\n\x07\n"),
			o:     defaultOptions(),
			want:  "(no printable text)\n",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, err := squeeze(tt.input, tt.o)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, r.Text); diff != "" {
				t.Errorf("text (-want +got):\n%s", diff)
			}
			checkAccounting(t, r.Data.(report))
		})
	}
}

func TestOneLineJSON(t *testing.T) {
	t.Parallel()
	long := `{"items":[` + strings.Repeat(`{"id":1,"msg":"ok"},`, 40) + `{"id":2,"msg":"error: boom"}]}`
	tests := map[string]struct {
		input    string
		o        options
		indented bool
	}{
		"a long document":          {long + "\n", defaultOptions(), true},
		"a short document":         {`{"a":1}` + "\n", defaultOptions(), false},
		"no byte limit":            {long + "\n", options{MaxLines: 200}, false},
		"two documents":            {long + "\n" + long + "\n", defaultOptions(), false},
		"invalid JSON":             {long[:len(long)-1] + "\n", defaultOptions(), false},
		"already on several lines": {"{\n" + `"a":` + strings.Repeat("1", 600) + "\n}\n", defaultOptions(), false},
	}
	for name, tt := range tests {
		r, err := squeeze([]byte(tt.input), tt.o)
		if err != nil {
			t.Fatal(err)
		}
		rep := r.Data.(report)
		if rep.JSONIndented != tt.indented {
			t.Errorf("%s: json_indented = %v, want %v", name, rep.JSONIndented, tt.indented)
		}
		checkAccounting(t, rep)
		if tt.indented && (!strings.Contains(r.Text, "\"msg\": \"error: boom\"\n") || strings.Contains(r.Text, "more bytes")) {
			t.Errorf("%s: text:\n%s", name, r.Text)
		}
	}
}

func TestReport(t *testing.T) {
	t.Parallel()
	o := options{MaxLines: 5, MaxLineBytes: 4}
	r := build(parse(lines("", "a", "a", "a", "b", "", "", strings.Repeat("z", 9), "Error x", "c", "d"), o.MaxLineBytes), o)
	want := report{Tool: "text", Lines: 11, Printed: 5, BlankDropped: 2, Folded: 2, Cut: 2, LongLines: 2, ErrorLines: 1}
	if diff := cmp.Diff(want, r, cmpopts.IgnoreUnexported(report{})); diff != "" {
		t.Errorf("report (-want +got):\n%s", diff)
	}
}

func checkAccounting(t *testing.T, r report) {
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
		formattest.CheckStable(t, func() (core.Result, error) { return squeeze(input, defaultOptions()) })
		for _, o := range []options{defaultOptions(), {MaxLines: 3, MaxLineBytes: 2}, {NoFold: true, MaxLines: 1}} {
			res, err := squeeze(input, o)
			if err != nil {
				t.Fatal(err)
			}
			formattest.CheckText(t, res.Text)
			if strings.ContainsRune(res.Text, 0x1b) {
				t.Fatalf("escape byte in output: %q", res.Text)
			}
			r := res.Data.(report)
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
			if utf8.Valid(input) && !r.JSONIndented && r.Printed > 0 && r.Folded == 0 && r.Cut == 0 && r.LongLines == 0 && len(res.Text) > len(input)+1 {
				t.Fatalf("output %d bytes from %d input bytes with nothing folded or cut:\n%q", len(res.Text), len(input), res.Text)
			}
			if _, err := core.Envelope("generic", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

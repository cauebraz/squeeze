package tfdiag

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestConvert(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		raw  string
		want Diagnostic
	}{
		"full": {
			`{"severity":"error","summary":"Unsupported block type","detail":"Blocks of type \"resorce\" are not expected here.","address":"","range":{"filename":"main.tf","start":{"line":1,"column":1,"byte":0},"end":{"line":1,"column":8,"byte":7}},"snippet":{"context":null,"code":"resorce \"test_instance\" \"foo\" {","start_line":1,"highlight_start_offset":0,"highlight_end_offset":7,"values":[]}}`,
			Diagnostic{Severity: "error", Summary: "Unsupported block type", Detail: `Blocks of type "resorce" are not expected here.`, File: "main.tf", Line: 1, Column: 1, Code: `resorce "test_instance" "foo" {`},
		},
		"no range no snippet": {
			`{"severity":"warning","summary":"Deprecated","detail":"","address":"aws_s3_bucket.b"}`,
			Diagnostic{Severity: "warning", Summary: "Deprecated", Address: "aws_s3_bucket.b"},
		},
		"empty severity is an error": {
			`{"summary":"x"}`,
			Diagnostic{Severity: "error", Summary: "x"},
		},
		"unknown severity is an error": {
			`{"severity":"0","summary":"x"}`,
			Diagnostic{Severity: "error", Summary: "x"},
		},
		"multi-line snippet picks the highlighted line": {
			`{"severity":"error","summary":"s","snippet":{"code":"  a = 1\n  b = var.missing\n","start_line":4,"highlight_start_offset":12,"highlight_end_offset":23}}`,
			Diagnostic{Severity: "error", Summary: "s", Code: "b = var.missing"},
		},
		"highlight past the code": {
			`{"severity":"error","summary":"s","snippet":{"code":"x","start_line":1,"highlight_start_offset":99}}`,
			Diagnostic{Severity: "error", Summary: "s", Code: "x"},
		},
		"control characters removed": {
			"{\"severity\":\"error\",\"summary\":\"bad\\u001b[31m color\\tand tab\"}",
			Diagnostic{Severity: "error", Summary: "bad color and tab"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var raw Raw
			if err := json.Unmarshal([]byte(tt.raw), &raw); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, Convert(raw)); diff != "" {
				t.Errorf("Convert() (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCodeLineTruncated(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 200)
	d := Convert(Raw{Summary: "s", Snippet: &Snippet{Code: long}})
	if len(d.Code) != maxCodeBytes || !strings.HasSuffix(d.Code, "...") {
		t.Errorf("Code = %d bytes %q", len(d.Code), d.Code[len(d.Code)-5:])
	}
}

func TestFoldAndSort(t *testing.T) {
	t.Parallel()
	w := Diagnostic{Severity: "warning", Summary: "Deprecated attribute", File: "b.tf", Line: 3}
	e1 := Diagnostic{Severity: "error", Summary: "Missing", File: "b.tf", Line: 9}
	e2 := Diagnostic{Severity: "error", Summary: "Bad", File: "a.tf", Line: 20}
	e3 := Diagnostic{Severity: "error", Summary: "Bad", File: "a.tf", Line: 2, Column: 5}
	other := Diagnostic{Severity: "info", Summary: "z"}
	got := Normalize([]Diagnostic{w, e1, w, other, e2, w, e3})
	wantW := w
	wantW.Count = 3
	want := []Diagnostic{e3, e2, e1, wantW, other}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Normalize() (-want +got):\n%s", diff)
	}
	errs, warns := Count(got)
	if errs != 3 || warns != 3 {
		t.Errorf("Count() = %d, %d; want 3, 3", errs, warns)
	}
}

func TestSortTieBreakers(t *testing.T) {
	t.Parallel()
	ds := []Diagnostic{
		{Severity: "error", Summary: "b", Address: "y"},
		{Severity: "error", Summary: "b", Address: "x"},
		{Severity: "error", Summary: "a", Address: "z"},
	}
	Sort(ds)
	got := []string{ds[0].Summary + ds[0].Address, ds[1].Summary + ds[1].Address, ds[2].Summary + ds[2].Address}
	if diff := cmp.Diff([]string{"az", "bx", "by"}, got); diff != "" {
		t.Errorf("Sort() (-want +got):\n%s", diff)
	}
}

func TestWrite(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("word ", 80)
	tests := map[string]struct {
		d    Diagnostic
		o    WriteOptions
		want string
	}{
		"full": {
			Diagnostic{Severity: "error", Summary: "Unsupported block type", Detail: "Blocks of type \"resorce\"\nare not expected here.", File: "main.tf", Line: 1, Column: 1, Code: `resorce "x" "y" {`},
			WriteOptions{},
			"error: Unsupported block type (main.tf:1:1)\n  Blocks of type \"resorce\" are not expected here.\n  > resorce \"x\" \"y\" {\n",
		},
		"address and count": {
			Diagnostic{Severity: "warning", Summary: "Deprecated", File: "b.tf", Line: 3, Address: "aws_s3_bucket.b", Count: 2},
			WriteOptions{},
			"warning: Deprecated (b.tf:3, aws_s3_bucket.b) (x2)\n",
		},
		"file without line": {
			Diagnostic{Severity: "error", Summary: "s", File: "main.tf"},
			WriteOptions{},
			"error: s (main.tf)\n",
		},
		"indent and no detail": {
			Diagnostic{Severity: "error", Summary: "s", Detail: "hidden", Code: "x = 1"},
			WriteOptions{Indent: "  ", NoDetail: true},
			"  error: s\n    > x = 1\n",
		},
		"long detail is wrapped and cut": {
			Diagnostic{Severity: "error", Summary: "s", Detail: long},
			WriteOptions{},
			"error: s\n  " + strings.TrimSpace(strings.Repeat("word ", 20)) + "\n  " + strings.TrimSpace(strings.Repeat("word ", 20)) + "\n  " + strings.TrimSpace(strings.Repeat("word ", 19)) + "...\n",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var b strings.Builder
			Write(&b, tt.d, tt.o)
			if diff := cmp.Diff(tt.want, b.String()); diff != "" {
				t.Errorf("Write() (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWrapAndTruncate(t *testing.T) {
	t.Parallel()
	if got := wrap("", 10, 3); got != nil {
		t.Errorf("wrap(empty) = %q", got)
	}
	if got := wrap("averyveryverylongword short", 10, 3); !cmp.Equal(got, []string{"averyveryverylongword", "short"}) {
		t.Errorf("wrap(long word) = %q", got)
	}
	if got := truncate("abc", 5); got != "abc" {
		t.Errorf("truncate(short) = %q", got)
	}
	if got := truncate("cafécafé", 7); got != "caf..." {
		t.Errorf("truncate(utf8) = %q", got)
	}
	if got := cutWords("one two three", 9); got != "one two" {
		t.Errorf("cutWords(mid word) = %q", got)
	}
	if got := cutWords("one two three", 8); got != "one two" {
		t.Errorf("cutWords(at space) = %q", got)
	}
	if got := cutWords("onetwothree", 5); got != "onetw" {
		t.Errorf("cutWords(one word) = %q", got)
	}
	if got := cutWords("short", 9); got != "short" {
		t.Errorf("cutWords(short) = %q", got)
	}
	if got := cut("abc", 0); got != "" {
		t.Errorf("cut(0) = %q", got)
	}
}

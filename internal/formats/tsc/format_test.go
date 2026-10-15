package tsc

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
	"github.com/cauebraz/squeeze/internal/tfdiag"
	"github.com/google/go-cmp/cmp"
)

const fixtureDir = "testdata"

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name, "input.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func report(t *testing.T, input []byte, o Options) Report {
	t.Helper()
	r, err := Squeeze(input, o)
	if err != nil {
		t.Fatal(err)
	}
	return r.Data.(Report)
}

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
	tests := map[string]struct {
		input string
		want  bool
	}{
		"plain":              {"src/a.ts(1,7): error TS2322: Type 'string' is not assignable to type 'number'.\n", true},
		"pretty":             {"\x1b[96msrc/a.ts\x1b[0m:\x1b[93m1\x1b[0m:\x1b[93m7\x1b[0m - \x1b[91merror\x1b[0m\x1b[90m TS2322: \x1b[0mType\n", true},
		"global":             {"error TS5023: Unknown compiler option '--x'.\n", true},
		"windows path":       {"C:\\shop\\src\\a.ts(1,7): error TS2322: x\r\n", true},
		"npm banner":         {"\n> shop@1.0.0 typecheck\n> tsc --noEmit\n\nsrc/a.ts(1,7): error TS2322: x\n", true},
		"build verbose":      {"10:02:00 pm - Projects in this build: \n    * tsconfig.json\n\n10:02:00 pm - Building project 'tsconfig.json'...\n\nsrc/a.ts(1,7): error TS2322: x\n", true},
		"clean build":        {"[10:02:05 PM] Projects in this build: \r\n    * tsconfig.json\n", true},
		"watch":              {"22:02:05 - Starting compilation in watch mode...\n\n", true},
		"status only":        {"10:02:00 pm - something else\n", false},
		"text first":         {"hello\nsrc/a.ts(1,7): error TS2322: x\n", false},
		"no code":            {"src/a.ts(1,7): error: x\n", false},
		"gcc":                {"main.c:1:7: error: expected ';'\n", false},
		"eslint stylish":     {"/w/a.js\n  1:1  error  x  no-var\n", false},
		"go test":            {"ok  \texample.com/p\t0.1s\n", false},
		"other category":     {"src/a.ts(1,7): fatal TS2322: x\n", false},
		"position not first": {"src/a.ts: error TS2322 (1,7): x\n", false},
	}
	for name, tt := range tests {
		if got := (Format{}).Detect([]byte(tt.input)); got != tt.want {
			t.Errorf("%s: Detect() = %v, want %v", name, got, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"text":       "hello\nworld\n",
		"npm banner": "> shop@1.0.0 typecheck\n> tsc --noEmit\n",
	} {
		if _, err := Squeeze([]byte(input), DefaultOptions()); !errors.Is(err, ErrNotTSC) {
			t.Errorf("%s: Squeeze() error = %v, want ErrNotTSC", name, err)
		}
	}
}

func TestPlain(t *testing.T) {
	t.Parallel()
	r := report(t, fixture(t, "plain"), DefaultOptions())
	if r.Errors != 9 || r.Warnings != 0 || r.Files != 3 || r.Summary != "" || r.SummaryMismatch {
		t.Errorf("counts = %+v", r)
	}
	want := []core.Count{{Name: "TS2322", Count: 6}, {Name: "TS2307", Count: 1}, {Name: "TS2339", Count: 1}, {Name: "TS7006", Count: 1}}
	if diff := cmp.Diff(want, r.Codes); diff != "" {
		t.Errorf("codes (-want +got):\n%s", diff)
	}
	var handler *tfdiag.Diagnostic
	for i, d := range r.Diagnostics {
		if d.File == "src/order.ts" && d.Line == 6 {
			handler = &r.Diagnostics[i]
		}
	}
	wantDetail := strings.Join([]string{
		"Types of parameters 'event' and 'event' are incompatible.",
		"Type '{ id: number; payload: { items: Item[]; }; }' is not assignable to type '{ id: string; payload: { items: string[]; }; }'.",
		"Types of property 'id' are incompatible.",
		"Type 'number' is not assignable to type 'string'.",
	}, "\n")
	if handler == nil || handler.Detail != wantDetail || handler.Column != 7 || handler.Address != "TS2322" {
		t.Errorf("message chain = %+v", handler)
	}
}

func strip(ds []tfdiag.Diagnostic) []tfdiag.Diagnostic {
	out := make([]tfdiag.Diagnostic, 0, len(ds))
	for _, d := range ds {
		d.Values = nil
		out = append(out, d)
	}
	return out
}

func TestPrettyReadsTheSameDiagnostics(t *testing.T) {
	t.Parallel()
	plain := report(t, fixture(t, "plain"), DefaultOptions())
	for _, name := range []string{"pretty", "build-pretty", "ts7-build-pretty"} {
		r := report(t, fixture(t, name), DefaultOptions())
		if diff := cmp.Diff(plain.Diagnostics, strip(r.Diagnostics)); diff != "" {
			t.Errorf("%s: diagnostics differ from the plain run (-plain +pretty):\n%s", name, diff)
		}
		if len(r.Other) != 0 {
			t.Errorf("%s: lines left over: %q", name, r.Other)
		}
	}
}

func TestRelatedInformation(t *testing.T) {
	t.Parallel()
	want := map[int][]string{
		17: {"src/cart.ts:6:25: The expected type comes from property 'weight' which is declared here on type '{ name: string; weight: number; }'"},
		18: {"src/cart.ts:5:3: The expected type comes from property 'price' which is declared here on type 'Item'"},
	}
	for _, name := range []string{"pretty", "ts7-build-pretty"} {
		got := map[int][]string{}
		for _, d := range report(t, fixture(t, name), DefaultOptions()).Diagnostics {
			if len(d.Values) > 0 {
				got[d.Line] = d.Values
			}
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%s: related information (-want +got):\n%s", name, diff)
		}
	}
	input := "src/a.ts:1:7 - error TS2322: x\n\n1 a\n  ~\n\n    only a message\n"
	r := report(t, []byte(input), DefaultOptions())
	if diff := cmp.Diff([]string{"only a message"}, r.Diagnostics[0].Values); diff != "" {
		t.Errorf("related without a file (-want +got):\n%s", diff)
	}
}

func TestSummary(t *testing.T) {
	t.Parallel()
	type summary struct {
		Text     string
		Errors   int
		Files    int
		Mismatch bool
	}
	tests := map[string]summary{
		"pretty":           {"Found 9 errors in 3 files.", 9, 3, false},
		"ts7-build-pretty": {"Found 9 errors in 3 files.", 9, 3, false},
		"build-pretty":     {"Found 9 errors.", 9, 0, false},
		"config":           {"Found 2 errors in the same file, starting at: tsconfig.json:3", 2, 1, false},
		"one-file":         {"Found 1 error in src/lib/format.ts:6", 1, 1, false},
		"mismatch":         {"Found 9 errors in 3 files.", 9, 3, true},
		"plain":            {},
	}
	for name, want := range tests {
		r := report(t, fixture(t, name), DefaultOptions())
		got := summary{r.Summary, r.SummaryErrors, r.SummaryFiles, r.SummaryMismatch}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%s: summary (-want +got):\n%s", name, diff)
		}
	}
	for line, want := range map[string][2]int{
		"Found 1 error.": {1, 0},
		"[10:02:05 PM] Found 3 errors. Watching for file changes.": {3, 0},
		"Found 4 errors in 2 files.":                               {4, 2},
	} {
		r := report(t, []byte("src/a.ts(1,1): error TS1: x\n\n"+line+"\n"), DefaultOptions())
		if r.SummaryErrors != want[0] || r.SummaryFiles != want[1] {
			t.Errorf("%q: summary = %d errors in %d files, want %v", line, r.SummaryErrors, r.SummaryFiles, want)
		}
	}
	mismatch := report(t, []byte("a.ts(1,1): error TS1: x\nb.ts(1,1): error TS1: x\n\nFound 2 errors in 3 files.\n"), DefaultOptions())
	if !mismatch.SummaryMismatch {
		t.Error("a file count that differs is not flagged")
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"plain":       {"error", "fail"},
		"categories":  {"error", "fail", "warning"},
		"clean-build": {},
	}
	for name, want := range tests {
		r, err := Squeeze(fixture(t, name), DefaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, r.Conditions); diff != "" {
			t.Errorf("%s: conditions (-want +got):\n%s", name, diff)
		}
	}
	r, err := Squeeze([]byte("a.ts(1,1): message TS6133: x\n"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"fail"}, r.Conditions); diff != "" {
		t.Errorf("message only: tsc exits 1 on any diagnostic (-want +got):\n%s", diff)
	}
}

func TestCategories(t *testing.T) {
	t.Parallel()
	r := report(t, fixture(t, "categories"), DefaultOptions())
	if r.Errors != 2 || r.Warnings != 1 || r.Messages != 1 || r.Files != 1 {
		t.Errorf("counts = %+v", r)
	}
	if d := r.Diagnostics[0]; d.File != "" || d.Address != "TS6053" {
		t.Errorf("global diagnostic is not first: %+v", d)
	}
}

func TestLimitNamesTheOmittedCodes(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "plain"), Options{MaxErrors: 3})
	if err != nil {
		t.Fatal(err)
	}
	data := r.Data.(Report)
	if len(data.Diagnostics) != 3 || data.ErrorsOmitted != 6 {
		t.Fatalf("diagnostics = %d, omitted %d", len(data.Diagnostics), data.ErrorsOmitted)
	}
	want := "... 6 more errors, TS2322 3, TS2307 1, TS2339 1, TS7006 1 (--max-errors 0)\n"
	if !strings.HasSuffix(r.Text, want) {
		t.Errorf("text:\n%s\nwant suffix:\n%s", r.Text, want)
	}
}

func TestUnknownLines(t *testing.T) {
	t.Parallel()
	input := "src/a.ts(1,1): error TS1005: ';' expected.\n" +
		"npm error Lifecycle script `typecheck` failed with error:\n" +
		"some note\n" +
		"10:00:00 pm - Project 'b' can't be built because its dependency 'a' has errors\n" +
		"deprecated: this API warns\n"
	r := report(t, []byte(input), DefaultOptions())
	var got []string
	for _, d := range r.Diagnostics {
		got = append(got, d.Severity+" "+d.Address+" "+d.Summary)
	}
	want := []string{
		"error  npm error Lifecycle script `typecheck` failed with error:",
		"error TS1005 ';' expected.",
		"warning  deprecated: this API warns",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("diagnostics (-want +got):\n%s", diff)
	}
	wantOther := []string{"some note", "10:00:00 pm - Project 'b' can't be built because its dependency 'a' has errors"}
	if diff := cmp.Diff(wantOther, r.Other); diff != "" {
		t.Errorf("other lines (-want +got):\n%s", diff)
	}
	if text := Text(r); !strings.HasSuffix(text, "... 2 other lines (--all)\n") {
		t.Errorf("other lines cut without a notice:\n%s", text)
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := "src/a\x1b[2K.ts(1,1): error TS1: m\x1b]0;x\x07\u2028error: forged\n" +
		"  chain\u2028error: forged\n" +
		"\x1b[31merror: forged TS\n"
	for _, o := range []Options{DefaultOptions(), {All: true}} {
		r, err := Squeeze([]byte(input), o)
		if err != nil {
			t.Fatal(err)
		}
		formattest.CheckText(t, r.Text)
		for _, line := range strings.Split(r.Text, "\n") {
			if strings.HasPrefix(line, "error: forged") && !strings.HasPrefix(line, "error: forged TS") {
				t.Errorf("input forged a line: %q\n%s", line, r.Text)
			}
		}
	}
}

func TestDataPayload(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "clean-build"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	out, err := core.Envelope("tsc", 10, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "null") {
		t.Errorf("envelope has a null collection: %s", out)
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte("a.ts:1:1 - error TS1: x\n  y\n\n  b.ts:2:2\n    3 z\n      ~\n    m\n\nFound 2 errors.\nErrors  Files\n     1  a.ts:1\n"))
	f.Fuzz(func(t *testing.T, input []byte) {
		l, err := Parse(input)
		if err != nil {
			return
		}
		for _, o := range []Options{DefaultOptions(), {MaxErrors: 1}, {All: true}} {
			r := Build(l, o)
			res := core.Result{Text: Text(r), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			shown := 0
			for _, d := range r.Diagnostics {
				shown += max(d.Count, 1)
			}
			if shown+r.ErrorsOmitted != len(l.Diagnostics) || r.Errors+r.Warnings+r.Messages != len(l.Diagnostics) {
				t.Fatalf("%d diagnostics shown and %d omitted, %d counted, %d parsed", shown, r.ErrorsOmitted, r.Errors+r.Warnings+r.Messages, len(l.Diagnostics))
			}
			if o.MaxErrors > 0 && len(r.Diagnostics) > o.MaxErrors {
				t.Fatalf("%d lines over the limit %d", len(r.Diagnostics), o.MaxErrors)
			}
			if r.ErrorsOmitted > 0 && !strings.Contains(res.Text, "(--max-errors 0)") {
				t.Fatalf("diagnostics cut without a notice:\n%s", res.Text)
			}
			if !o.All && len(r.Other) > 0 && !strings.Contains(res.Text, "(--all)") {
				t.Fatalf("other lines cut without a notice:\n%s", res.Text)
			}
			if _, err := core.Envelope("tsc", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

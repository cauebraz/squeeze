package gotest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
	"github.com/google/go-cmp/cmp"
)

const fixtureDir = "testdata"

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name, "input.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func stream(lines ...string) []byte {
	return []byte(strings.Join(lines, "\n") + "\n")
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
		"start":         {`{"Action":"start","Package":"p"}`, true},
		"build output":  {`{"ImportPath":"p","Action":"build-output","Output":"x\n"}`, true},
		"after text":    {"go: downloading x v1\n{\"Action\":\"run\",\"Package\":\"p\",\"Test\":\"T\"}", true},
		"unknown":       {`{"Action":"explode","Package":"p"}`, false},
		"no package":    {`{"Action":"start"}`, false},
		"tfjson stream": {`{"@level":"info","@message":"x","type":"version"}`, false},
		"plain text":    {"ok  \tp\t0.1s\n", false},
		"after module downloads": {strings.Repeat("go: downloading example.com/m v1.0.0\n", 12) +
			"# example.com/p\n" + `{"Action":"start","Package":"p"}`, true},
		"after twenty text lines": {strings.Repeat("x\n", 20) + `{"Action":"start","Package":"p"}`, false},
	}
	for name, tt := range tests {
		if got := (Format{}).Detect([]byte(tt.input)); got != tt.want {
			t.Errorf("%s: Detect() = %v, want %v", name, got, tt.want)
		}
	}
}

func TestParseRejectsInputWithoutEvents(t *testing.T) {
	t.Parallel()
	if _, err := Squeeze([]byte("ok  \tp\t0.1s\n"), DefaultOptions()); err == nil {
		t.Error("Squeeze() returned no error")
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"mixed":      {"build_failed", "error", "fail", "panic", "skip"},
		"timeout":    {"error", "fail", "timeout"},
		"race":       {"error", "fail", "race"},
		"bench":      {},
		"pass":       {},
		"legacy":     {"build_failed", "error", "fail"},
		"unfinished": {"error", "fail"},
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
}

func TestCounts(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "mixed"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	report := r.Data.(Report)
	if diff := cmp.Diff(Counts{Passed: 4, Failed: 3, Skipped: 1}, report.Tests); diff != "" {
		t.Errorf("tests (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(PackageCounts{OK: 1, Failed: 1, BuildFailed: 1, NoTests: 1}, report.Packages); diff != "" {
		t.Errorf("packages (-want +got):\n%s", diff)
	}
	var names []string
	for _, f := range report.Failures {
		names = append(names, f.Test)
	}
	if diff := cmp.Diff([]string{"TestAddNegative", "TestDivByZero", "TestTable/one"}, names); diff != "" {
		t.Errorf("failures (-want +got):\n%s", diff)
	}
}

func TestParentWithOwnOutputIsListed(t *testing.T) {
	t.Parallel()
	input := stream(
		`{"Action":"run","Package":"p","Test":"TestA"}`,
		`{"Action":"output","Package":"p","Test":"TestA","Output":"    a_test.go:3: setup failed\n","OutputType":"error"}`,
		`{"Action":"run","Package":"p","Test":"TestA/x"}`,
		`{"Action":"fail","Package":"p","Test":"TestA/x"}`,
		`{"Action":"fail","Package":"p","Test":"TestA"}`,
		`{"Action":"run","Package":"p","Test":"TestAB"}`,
		`{"Action":"fail","Package":"p","Test":"TestAB"}`,
		`{"Action":"fail","Package":"p"}`,
	)
	r, err := Squeeze(input, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range r.Data.(Report).Failures {
		names = append(names, f.Test)
	}
	if diff := cmp.Diff([]string{"TestA", "TestA/x", "TestAB"}, names); diff != "" {
		t.Errorf("failures (-want +got):\n%s", diff)
	}
}

func TestErrorLinesHideLogsButKeepPanics(t *testing.T) {
	t.Parallel()
	input := stream(
		`{"Action":"run","Package":"p","Test":"TestA"}`,
		`{"Action":"output","Package":"p","Test":"TestA","Output":"    a_test.go:3: log line\n"}`,
		`{"Action":"output","Package":"p","Test":"TestA","Output":"    a_test.go:4: got 1\n","OutputType":"error"}`,
		`{"Action":"output","Package":"p","Test":"TestA","Output":"panic: boom\n"}`,
		`{"Action":"fail","Package":"p","Test":"TestA"}`,
		`{"Action":"fail","Package":"p"}`,
	)
	r, err := Squeeze(input, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	got := r.Data.(Report).Failures[0].Output
	if diff := cmp.Diff(Output{Lines: []string{"    a_test.go:4: got 1", "panic: boom"}, Logs: 1}, got); diff != "" {
		t.Errorf("output (-want +got):\n%s", diff)
	}
	logs, err := Squeeze(input, Options{Full: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := logs.Data.(Report).Failures[0].Output; len(got.Lines) != 3 || got.Logs != 0 {
		t.Errorf("--full output = %+v", got)
	}
}

func TestEveryRunOfARepeatedTestIsCounted(t *testing.T) {
	t.Parallel()
	input := stream(
		`{"Action":"run","Package":"p","Test":"TestFlaky"}`,
		`{"Action":"output","Package":"p","Test":"TestFlaky","Output":"    f_test.go:9: boom\n","OutputType":"error"}`,
		`{"Action":"fail","Package":"p","Test":"TestFlaky"}`,
		`{"Action":"run","Package":"p","Test":"TestFlaky"}`,
		`{"Action":"pass","Package":"p","Test":"TestFlaky"}`,
		`{"Action":"fail","Package":"p"}`,
	)
	r, err := Squeeze(input, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	report := r.Data.(Report)
	if diff := cmp.Diff(Counts{Passed: 1, Failed: 1}, report.Tests); diff != "" {
		t.Errorf("tests (-want +got):\n%s", diff)
	}
	if len(report.Failures) != 1 || report.Failures[0].Run != 1 || len(report.Failures[0].Output.Lines) != 1 {
		t.Fatalf("failures = %+v", report.Failures)
	}
	if !strings.Contains(r.Text, "fail p TestFlaky (run 1 of 2)\n  f_test.go:9: boom\n") {
		t.Errorf("text:\n%s", r.Text)
	}
}

func TestTypedErrorLinesAreNeverFraming(t *testing.T) {
	t.Parallel()
	input := stream(
		`{"Action":"run","Package":"p","Test":"TestA"}`,
		`{"Action":"output","Package":"p","Test":"TestA","Output":"    a_test.go:9: child go test failed:\n","OutputType":"error"}`,
		`{"Action":"output","Package":"p","Test":"TestA","Output":"        --- FAIL: TestInner (0.00s)\n","OutputType":"error-continue"}`,
		`{"Action":"output","Package":"p","Test":"TestA","Output":"--- FAIL: TestA (0.00s)\n","OutputType":"frame"}`,
		`{"Action":"fail","Package":"p","Test":"TestA"}`,
		`{"Action":"fail","Package":"p"}`,
	)
	for _, o := range []Options{DefaultOptions(), {Full: true}} {
		r, err := Squeeze(input, o)
		if err != nil {
			t.Fatal(err)
		}
		got := r.Data.(Report).Failures[0].Output.Lines
		if diff := cmp.Diff([]string{"a_test.go:9: child go test failed:", "    --- FAIL: TestInner (0.00s)"}, got); diff != "" {
			t.Errorf("full=%v lines (-want +got):\n%s", o.Full, diff)
		}
	}
}

func TestGoWarningsAreKept(t *testing.T) {
	t.Parallel()
	input := stream(
		`{"Action":"start","Package":"p"}`,
		`{"Action":"output","Package":"p","Output":"testing: warning: no tests to run\n"}`,
		`{"Action":"output","Package":"p","Output":"PASS\n","OutputType":"frame"}`,
		`{"Action":"output","Package":"p","Output":"ok  \tp\t0.01s\n"}`,
		`{"Action":"pass","Package":"p"}`,
	)
	r, err := Squeeze(input, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"p: testing: warning: no tests to run"}, r.Data.(Report).Warnings); diff != "" {
		t.Errorf("warnings (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"warning"}, r.Conditions); diff != "" {
		t.Errorf("conditions (-want +got):\n%s", diff)
	}
	if !strings.Contains(r.Text, "warning p: testing: warning: no tests to run\n") {
		t.Errorf("text:\n%s", r.Text)
	}
}

func TestCoverageIsKept(t *testing.T) {
	t.Parallel()
	input := stream(
		`{"Action":"output","Package":"a","Output":"coverage: 45.0% of statements in ./...\n"}`,
		`{"Action":"output","Package":"a","Output":"ok  \ta\t0.1s\tcoverage: 45.0% of statements in ./...\n"}`,
		`{"Action":"pass","Package":"a"}`,
		`{"Action":"output","Package":"b","Output":"ok  \tb\t0.1s\tcoverage: [no statements]\n"}`,
		`{"Action":"pass","Package":"b"}`,
		`{"Action":"output","Package":"c","Output":"ok  \tc\t0.1s\n"}`,
		`{"Action":"pass","Package":"c"}`,
	)
	r, err := Squeeze(input, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	want := []PackageCoverage{{Package: "a", Coverage: "45.0% of statements in ./..."}, {Package: "b", Coverage: "[no statements]"}}
	if diff := cmp.Diff(want, r.Data.(Report).Coverage); diff != "" {
		t.Errorf("coverage (-want +got):\n%s", diff)
	}
	if !strings.Contains(r.Text, "coverage a: 45.0% of statements in ./...\ncoverage b: [no statements]\n") {
		t.Errorf("text:\n%s", r.Text)
	}
	var many []string
	for i := range maxCoverage + 3 {
		p := fmt.Sprintf("p%02d", i)
		many = append(many, `{"Action":"output","Package":"`+p+`","Output":"coverage: 10.0% of statements\n"}`, `{"Action":"pass","Package":"`+p+`"}`)
	}
	r, err = Squeeze(stream(many...), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Data.(Report); len(got.Coverage) != maxCoverage || got.CoverageOmitted != 3 || !strings.Contains(r.Text, "... 3 more packages with coverage (--all)\n") {
		t.Errorf("coverage = %d, omitted = %d, text:\n%s", len(got.Coverage), got.CoverageOmitted, r.Text)
	}
	r, err = Squeeze(stream(many...), Options{All: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Data.(Report); len(got.Coverage) != maxCoverage+3 || got.CoverageOmitted != 0 {
		t.Errorf("--all coverage = %d, omitted = %d", len(got.Coverage), got.CoverageOmitted)
	}
}

func TestSplitOutputIsJoined(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 1500)
	input := stream(
		`{"Action":"run","Package":"p","Test":"TestA"}`,
		`{"Action":"output","Package":"p","Test":"TestA","Output":"    a_test.go:3: `+long[:1000]+`","OutputType":"error"}`,
		`{"Action":"output","Package":"p","Test":"TestA","Output":"`+long[1000:]+`\nnext\n","OutputType":"error"}`,
		`{"Action":"output","Package":"p","Test":"TestA","Output":"tail without newline","OutputType":"error"}`,
		`{"Action":"fail","Package":"p","Test":"TestA"}`,
		`{"Action":"fail","Package":"p"}`,
	)
	r, err := Squeeze(input, Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"    a_test.go:3: " + long, "next", "tail without newline"}
	if diff := cmp.Diff(want, r.Data.(Report).Failures[0].Output.Lines); diff != "" {
		t.Errorf("lines (-want +got):\n%s", diff)
	}
}

func TestFilterStack(t *testing.T) {
	t.Parallel()
	in := []string{
		"panic: boom",
		"",
		"goroutine 7 [running]:",
		"testing.tRunner.func1()",
		"\t/usr/local/go/src/testing/testing.go:1 +0x1",
		"shop/calc.Div(...)",
		"\t/home/dev/shop/calc/calc.go:5",
		"created by testing.(*T).Run in goroutine 1",
		"\t/usr/local/go/src/testing/testing.go:2 +0x2",
		"",
		"goroutine 1 [chan receive]:",
		"main.main()",
		"\t_testmain.go:48 +0x80",
		"",
		"goroutine 9 [select]:",
		"github.com/acme/lib.Wait()",
		"\t/home/dev/go/pkg/mod/github.com/acme/lib/wait.go:9 +0x9",
		"  example.com/x.F()",
		"      /home/dev/x/f.go:3 +0x3",
		"  runtime.goexit()",
		"      /usr/local/go/src/runtime/asm.s:1 +0x4",
	}
	got, dropped := filterStack(in, "shop/calc")
	want := []string{
		"panic: boom",
		"",
		"goroutine 7 [running]:",
		"shop/calc.Div(...)",
		"\t/home/dev/shop/calc/calc.go:5",
		"",
		"goroutine 9 [select]:",
		"github.com/acme/lib.Wait()",
		"\t/home/dev/go/pkg/mod/github.com/acme/lib/wait.go:9 +0x9",
		"  example.com/x.F()",
		"      /home/dev/x/f.go:3 +0x3",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("filterStack (-want +got):\n%s", diff)
	}
	if dropped != 9 {
		t.Errorf("dropped = %d, want 9", dropped)
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := stream(
		`{"Action":"run","Package":"p\nfail forged","Test":"T\nfail forged"}`,
		`{"Action":"output","Package":"p\nfail forged","Test":"T\nfail forged","Output":"x\u001b[31m\rfail forged\n","OutputType":"error"}`,
		`{"Action":"attr","Package":"p\nfail forged","Test":"T\nfail forged","Key":"k\nfail forged","Value":"v"}`,
		`{"Action":"fail","Package":"p\nfail forged","Test":"T\nfail forged"}`,
		`{"Action":"bench\nfail forged","Package":"p"}`,
	)
	for _, o := range []Options{DefaultOptions(), {All: true, Full: true}} {
		r, err := Squeeze(input, o)
		if err != nil {
			t.Fatal(err)
		}
		formattest.CheckText(t, r.Text)
		for _, line := range strings.Split(r.Text, "\n") {
			if strings.HasPrefix(line, "fail forged") {
				t.Errorf("input forged a line: %q\n%s", line, r.Text)
			}
		}
	}
}

func TestDataPayload(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "pass"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	out, err := core.Envelope("gotest", 10, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Data struct {
			Failures []any  `json:"failures"`
			Status   string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &doc); err != nil || doc.Data.Failures == nil || doc.Data.Status != "pass" {
		t.Errorf("envelope = %s, err = %v", out, err)
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add(stream(`{"Action":"run","Package":"a/b","Test":"T/x"}`, `{"Action":"output","Package":"a/b","Test":"T/x","Output":"goroutine 1 [x]:\nf(\n\tg.go:1\n"}`, `{"Action":"fail","Package":"a/b"}`))
	f.Fuzz(func(t *testing.T, input []byte) {
		events, err := Parse(input)
		if err != nil {
			return
		}
		failedEvents := 0
		for _, e := range events {
			if e.Action == "fail" {
				failedEvents++
			}
		}
		for _, o := range []Options{DefaultOptions(), {MaxTests: 1, MaxOutput: 1, All: true}, {Full: true}} {
			r := Build(events, o)
			res := core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			if failedEvents > 0 && r.Status != "fail" {
				t.Fatalf("%d fail events but status %q", failedEvents, r.Status)
			}
			if got := len(r.Failures) + r.FailuresOmitted; got != r.Tests.Failed {
				t.Fatalf("%d failures listed or omitted, %d counted", got, r.Tests.Failed)
			}
			if o.MaxOutput > 0 {
				for _, fl := range r.Failures {
					if len(fl.Output.Lines) > o.MaxOutput {
						t.Fatalf("%d output lines over the limit %d", len(fl.Output.Lines), o.MaxOutput)
					}
				}
			}
			if _, err := core.Envelope("gotest", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

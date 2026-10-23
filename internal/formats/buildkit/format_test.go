package buildkit

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
)

const fixtureDir = "testdata"

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(fixtureDir, name, "input.*"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("fixture %s: %v %v", name, matches, err)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func run(t *testing.T, input []byte, o options) (core.Result, report) {
	t.Helper()
	r, err := squeeze(input, o)
	if err != nil {
		t.Fatal(err)
	}
	return r, r.Data.(report)
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
		"buildx plain":        {"#0 building with \"default\" instance using docker driver\n\n#1 [internal] load build definition from Dockerfile\n", true},
		"compose service":     {"#1 [web internal] load build definition from Dockerfile\n", true},
		"older dockerignore":  {"#1 [internal] load .dockerignore\n#1 transferring context: 2B done\n", true},
		"rawjson":             {`{"vertexes":[{"digest":"sha256:f6","name":"[internal] load build definition from Dockerfile"}]}` + "\n", true},
		"rawjson logs first":  {`{"logs":[{"vertex":"sha256:f6","data":"aGkK"}]}` + "\n", true},
		"markdown heading":    {"#1 Introduction\n", false},
		"other json":          {`{"Time":"2026-10-13T22:20:23Z","Action":"start"}` + "\n", false},
		"github actions":      {"build\tUNKNOWN STEP\t2026-10-13T22:20:23.7522810Z #1 [internal] load build definition from Dockerfile\n", false},
		"go test text":        {"ok  \texample.com/p\t0.1s\n", false},
		"plain past 3 lines":  {"a\nb\nc\n#1 [internal] load build definition from Dockerfile\n", false},
		"legacy docker build": {"Sending build context to Docker daemon  2.048kB\nStep 1/2 : FROM alpine\n", false},
	}
	for name, tt := range tests {
		if got := (Format{}).Detect([]byte(tt.input)); got != tt.want {
			t.Errorf("%s: Detect() = %v, want %v", name, got, tt.want)
		}
	}
}

func TestParseRejectsInputWithoutProgress(t *testing.T) {
	t.Parallel()
	if _, err := squeeze([]byte("hello\n{\"a\":1}\n"), defaultOptions()); err == nil {
		t.Error("Squeeze() returned no error")
	}
}

func TestPlainAndRawJSONReadTheSame(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"failed-run", "missing-copy", "warning", "canceled"} {
		plain, pr := run(t, fixture(t, name), defaultOptions())
		raw, rr := run(t, fixture(t, name+"-rawjson"), defaultOptions())
		if pr.Progress != "plain" || rr.Progress != "rawjson" {
			t.Errorf("%s: progress %q and %q", name, pr.Progress, rr.Progress)
		}
		if name == "missing-copy" {
			continue
		}
		if diff := cmp.Diff(plain.Text, raw.Text); diff != "" {
			t.Errorf("%s: text differs (-plain +rawjson):\n%s", name, diff)
		}
	}
}

func TestStepStatuses(t *testing.T) {
	t.Parallel()
	_, r := run(t, fixture(t, "canceled-rawjson"), options{All: true})
	got := map[string]stepStatus{}
	for _, s := range r.Steps {
		got[s.Name[:min(len(s.Name), 16)]] = s.Status
	}
	want := map[string]stepStatus{
		"[internal] load ": statusDone,
		"[app 1/2] FROM d": statusCached,
		"[assets 2/2] RUN": statusCanceled,
		"[app 2/2] RUN ec": statusFailed,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: status %q, want %q (all %v)", k, got[k], v, got)
		}
	}
	if r.TotalSteps != 6 || r.CachedSteps != 1 || r.FailedSteps != 1 || r.CanceledSteps != 1 || r.OtherSteps != 0 {
		t.Errorf("counts %+v", r)
	}
}

func TestUnknownStatusPrintsAsIs(t *testing.T) {
	t.Parallel()
	got := text(report{Steps: []step{{Name: "[app 1/1] RUN make", Status: "paused"}}})
	want := "buildkit: 0 steps\npaused [app 1/1] RUN make\n"
	if got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func TestFailedStepPrintsItsLogTailAndTheFinalError(t *testing.T) {
	t.Parallel()
	res, r := run(t, fixture(t, "failed-run"), options{MaxLines: 3})
	want := "buildkit: 7 steps, 1 cached, 1 failed\n" +
		"fail [4/5] RUN for i in $(seq 1 25); do echo \"compiling module $i\"; done; echo \"error: module 26 failed: undefined symbol shop_init\" >&2; exit 1\n" +
		"  ... 23 lines cut (--max-lines 0)\n" +
		"  compiling module 24\n" +
		"  compiling module 25\n" +
		"  error: module 26 failed: undefined symbol shop_init\n" +
		"error: Dockerfile:4: failed to build: failed to solve: process \"/bin/sh -c for i in $(seq 1 25); do echo \\\"compiling module $i\\\"; done; echo \\\"error: module 26 failed: undefined symbol shop_init\\\" >&2; exit 1\" did not complete successfully: exit code: 1\n" +
		"... 6 other steps (--all)\n"
	if diff := cmp.Diff(want, res.Text); diff != "" {
		t.Errorf("text (-want +got):\n%s", diff)
	}
	if r.Steps[0].Error == "" || r.Steps[0].LinesOmitted != 23 || r.Location != "Dockerfile:4" {
		t.Errorf("step %+v, location %q", r.Steps[0], r.Location)
	}
}

func TestWarningsAreNeverDropped(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"warning", "warning-rawjson", "cached"} {
		res, r := run(t, fixture(t, name), defaultOptions())
		if len(r.Warnings) == 0 || !cmp.Equal(res.Conditions, []string{"warning"}) {
			t.Errorf("%s: conditions %v, warnings %v", name, res.Conditions, r.Warnings)
		}
		for _, w := range r.Warnings {
			if !strings.Contains(res.Text, "warning: "+w.Message+"\n") {
				t.Errorf("%s: warning %q not printed:\n%s", name, w.Message, res.Text)
			}
		}
	}
	_, r := run(t, fixture(t, "warning-rawjson"), defaultOptions())
	want := warning{
		Step:    "[internal] load build definition from Dockerfile",
		Message: "FromAsCasing: 'as' and 'FROM' keywords' casing do not match (line 1)",
		Detail:  []string{"The 'as' keyword should match the case of the 'from' keyword"},
		URL:     "https://docs.docker.com/go/dockerfile/rule/from-as-casing/",
	}
	if diff := cmp.Diff(want, r.Warnings[0]); diff != "" {
		t.Errorf("warning (-want +got):\n%s", diff)
	}
}

func TestWarningOnlyInTheSummaryIsKept(t *testing.T) {
	t.Parallel()
	input := "#1 [internal] load build definition from Dockerfile\n#1 WARN: A: first (line 1)\n#1 DONE 0.0s\n\n" +
		"WARNING: current commit information was not captured by the build\n" +
		" \x1b[33m2 warnings found (use docker --debug to expand):\n\x1b[0m - A: first (line 1)\n - B: second (line 2)\n"
	res, r := run(t, []byte(input), defaultOptions())
	var got []string
	for _, w := range r.Warnings {
		got = append(got, w.Message)
	}
	want := []string{"A: first (line 1)", "current commit information was not captured by the build", "B: second (line 2)"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("warnings (-want +got):\n%s\n%s", diff, res.Text)
	}
}

func TestUnknownLinesWithErrorsAreKept(t *testing.T) {
	t.Parallel()
	input := "#1 [internal] load build definition from Dockerfile\n#1 0.1 partial\ncontinued output\n#1 DONE 0.0s\n\n" +
		"------\n > x:\n------\nfatal: something broke\nplain note\n"
	res, r := run(t, []byte(input), options{All: true})
	if !cmp.Equal(r.Errors, []string{"fatal: something broke"}) || !strings.Contains(res.Text, "  continued output\n") {
		t.Errorf("errors %v, text:\n%s", r.Errors, res.Text)
	}
	if !cmp.Equal(res.Conditions, []string{"error", "fail"}) {
		t.Errorf("conditions %v", res.Conditions)
	}
}

func TestDockerfileSourceAndBuildArgsAreNotPrinted(t *testing.T) {
	t.Parallel()
	input := fixture(t, "warning-rawjson")
	if !strings.Contains(string(input), base64.StdEncoding.EncodeToString([]byte("FROM alpine:3.21 as base\nENV SHOP_SECRET=s3cr3t-value\nMAINTAINER dev@example.com\nRUN echo \"warn build\"\nCMD echo hello\n"))) {
		t.Fatal("fixture no longer carries the Dockerfile source in sourceInfo")
	}
	for _, o := range []options{defaultOptions(), {All: true}} {
		res, _ := run(t, input, o)
		envelope, err := core.Envelope("buildkit", len(input), res, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, out := range []string{res.Text, string(envelope)} {
			if strings.Contains(out, "s3cr3t-value") || strings.Contains(out, "MAINTAINER dev@example.com") {
				t.Errorf("Dockerfile source printed:\n%s", out)
			}
		}
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	log := base64.StdEncoding.EncodeToString([]byte("out\x1b[2Jput\rbad\x07\n"))
	short := base64.StdEncoding.EncodeToString([]byte("W\nforged: line\x1b[31m"))
	input := `{"vertexes":[{"digest":"d","name":"[1/1] RUN x\nfail forged\u001b]0;t\u0007","started":"2026-10-15T00:59:44Z","completed":"2026-10-15T00:59:45Z","error":"bad\nerror: forged"}]}` + "\n" +
		`{"logs":[{"vertex":"d","stream":1,"data":"` + log + `"}],"warnings":[{"vertex":"d","short":"` + short + `"}]}` + "\n" +
		"ERROR: failed\x1b[1m to solve\n"
	res, _ := run(t, []byte(input), options{All: true})
	formattest.CheckText(t, res.Text)
	for _, l := range strings.Split(res.Text, "\n") {
		if strings.HasPrefix(l, "fail forged") || strings.HasPrefix(l, "forged") || strings.HasPrefix(l, "error: forged") {
			t.Errorf("forged line %q in:\n%s", l, res.Text)
		}
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte("#1 [internal] load build definition from Dockerfile\n#1 WARN: x\n#2 a\n#2 ERROR: y\nERROR: z\n"))
	f.Fuzz(func(t *testing.T, input []byte) {
		formattest.CheckStable(t, func() (core.Result, error) { return squeeze(input, defaultOptions()) })
		b, err := parse(input)
		if err != nil {
			return
		}
		logged := map[string]bool{}
		for _, v := range b.byKey {
			for _, l := range v.logs {
				logged[l] = true
			}
		}
		for _, o := range []options{defaultOptions(), {MaxLines: 1}, {All: true}} {
			r := build(b, o)
			res := core.Result{Text: text(r), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			var notices []string
			for _, l := range strings.Split(res.Text, "\n") {
				if !strings.HasPrefix(l, "  ") {
					notices = append(notices, l)
				}
			}
			for _, s := range r.Steps {
				for _, l := range s.Output {
					if !logged[l] {
						notices = append(notices, l)
					}
				}
			}
			formattest.CheckNotices(t, strings.Join(notices, "\n"))
			if len(r.Warnings) != len(b.warnings) {
				t.Fatalf("%d warnings reported, %d parsed", len(r.Warnings), len(b.warnings))
			}
			for _, w := range r.Warnings {
				if !strings.Contains(res.Text, "warning: "+w.Message+"\n") {
					t.Fatalf("warning %q not printed:\n%s", w.Message, res.Text)
				}
			}
			for _, e := range r.Errors {
				if !strings.Contains(res.Text, e+"\n") {
					t.Fatalf("error %q not printed:\n%s", e, res.Text)
				}
			}
			for _, s := range r.Steps {
				if s.Status == statusFailed && !strings.Contains(res.Text, "fail "+s.Name+"\n") {
					t.Fatalf("failed step %q not printed:\n%s", s.Name, res.Text)
				}
			}
			if r.TotalSteps != len(r.Steps)+r.OtherSteps {
				t.Fatalf("%d steps, %d printed, %d other", r.TotalSteps, len(r.Steps), r.OtherSteps)
			}
			if _, err := core.Envelope("buildkit", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

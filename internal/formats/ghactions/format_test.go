package ghactions

import (
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
	data, err := os.ReadFile(filepath.Join(fixtureDir, name, "input.log"))
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

func line(job, text string) string {
	return job + "\tUNKNOWN STEP\t2026-10-13T22:20:23.7522810Z " + text
}

func TestDetect(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input string
		want  bool
	}{
		"gh run view":       {line("build", "a") + "\n" + line("build", "b") + "\n" + line("build", "c") + "\n", true},
		"byte order mark":   {"build\tSet up job\t\xef\xbb\xbf2026-10-13T22:20:23Z a\n", true},
		"one line":          {line("build", "a") + "\n", true},
		"timestamps only":   {"2026-10-13T22:20:23.7522810Z a\n2026-10-13T22:20:24.1Z b\n", false},
		"text first":        {"hello\n" + line("build", "a") + "\n", false},
		"tab separated tsv": {"a\tb\tc\n1\t2\t3\n", false},
		"go test text":      {"ok  \texample.com/p\t0.1s\n", false},
		"empty":             {"", false},
	}
	for name, tt := range tests {
		if got := (Format{}).Detect([]byte(tt.input)); got != tt.want {
			t.Errorf("%s: Detect() = %v, want %v", name, got, tt.want)
		}
	}
}

func TestParseRejectsInputWithoutTimestamps(t *testing.T) {
	t.Parallel()
	if _, err := Squeeze([]byte("hello\nworld\n"), DefaultOptions()); err == nil {
		t.Error("Squeeze() returned no error")
	}
}

func TestDownloadedJobLogWithoutColumns(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		"\xef\xbb\xbf2026-10-13T22:20:23.1Z Current runner version: '2.337.0'",
		"2026-10-13T22:20:24.1Z ##[group]Run make test",
		"2026-10-13T22:20:24.2Z make test",
		"2026-10-13T22:20:24.3Z ##[endgroup]",
		"2026-10-13T22:20:25.1Z \tmain_test.go:9: want 2",
		"2026-10-13T22:20:25.2Z ##[error]Process completed with exit code 2.",
	}, "\n")
	r, err := Squeeze([]byte(input), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	want := "github actions 2.337.0: 1 job, 1 failed; 1 error\n" +
		"fail job: make test\n" +
		"      main_test.go:9: want 2\n" +
		"  error: Process completed with exit code 2.\n"
	if r.Text != want {
		t.Errorf("text:\n%s\nwant:\n%s", r.Text, want)
	}
}

func TestMatrix(t *testing.T) {
	t.Parallel()
	r := report(t, fixture(t, "matrix"), DefaultOptions())
	var got []string
	for _, j := range r.Jobs {
		for _, s := range j.Steps {
			got = append(got, j.Name+": "+s.Name+": "+strings.Join(s.Errors, "|"))
		}
	}
	want := []string{
		"check (macos-latest, 1.27.2): just check: Process completed with exit code 1.",
		"check (macos-latest, 1.26.9): just check: Process completed with exit code 1.",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("failed steps (-want +got):\n%s", diff)
	}
	if r.ToolVersion != "2.337.0" || r.Errors != 2 || r.OtherSteps != 8 {
		t.Errorf("version %q, errors %d, other steps %d", r.ToolVersion, r.Errors, r.OtherSteps)
	}
	text, _ := Squeeze(fixture(t, "matrix"), DefaultOptions())
	for _, noise := range []string{"Runner Image", "GITHUB_TOKEN", "safe.directory", "GOTOOLCHAIN: local"} {
		if strings.Contains(text.Text, noise) {
			t.Errorf("setup line %q printed:\n%s", noise, text.Text)
		}
	}
}

func TestWarningsAreNeverDropped(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "warning"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	want := "step lint: golangci-lint run ./...\n  warning: golangci-lint reported 1 issue; the job continues because continue-on-error is set\n"
	if !strings.Contains(r.Text, want) || !cmp.Equal(r.Conditions, []string{"warning"}) {
		t.Errorf("conditions %v, text:\n%s", r.Conditions, r.Text)
	}
}

func TestAllPrintsEveryStep(t *testing.T) {
	t.Parallel()
	r := report(t, fixture(t, "warning"), Options{MaxLines: 30, All: true})
	var names []string
	for _, s := range r.Jobs[0].Steps {
		names = append(names, s.Name)
	}
	if r.OtherSteps != 0 || len(names) < 4 || names[0] != "set up job" || names[len(names)-1] != "post job cleanup" {
		t.Errorf("other %d, steps %v", r.OtherSteps, names)
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := line("j\u001b[31m", "##[group]Run x\u001b[2J") + "\n" + line("j\u001b[31m", "##[endgroup]") + "\n" +
		line("j\u001b[31m", "out\u0007put\u001b]0;title\u0007") + "\n" + line("j\u001b[31m", "##[error]bad\u001b[1m") + "\n"
	r, err := Squeeze([]byte(input), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	formattest.CheckText(t, r.Text)
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(line("j", "##[group]Run a") + "\n" + line("j", "##[error]x") + "\n" + line("", "##[warning]") + "\n"))
	f.Fuzz(func(t *testing.T, input []byte) {
		formattest.CheckStable(t, func() (core.Result, error) { return Squeeze(input, DefaultOptions()) })
		jobs, err := Parse(input)
		if err != nil {
			return
		}
		var messages []string
		for _, j := range jobs {
			for _, l := range j.Lines {
				if strings.HasPrefix(l.Text, "##[error]") {
					messages = append(messages, "  error: "+core.Clean(strings.TrimPrefix(l.Text, "##[error]"))+"\n")
				}
			}
		}
		errorsIn := len(messages)
		for _, o := range []Options{DefaultOptions(), {MaxLines: 1}, {All: true}} {
			r := Build(jobs, o)
			res := core.Result{Text: Text(r), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			if r.Errors != errorsIn {
				t.Fatalf("%d errors counted, %d in the input", r.Errors, errorsIn)
			}
			for _, m := range messages {
				if !strings.Contains(res.Text, m) {
					t.Fatalf("error %q not printed:\n%s", m, res.Text)
				}
			}
			if _, err := core.Envelope("ghactions", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

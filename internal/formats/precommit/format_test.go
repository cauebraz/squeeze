package precommit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
	"github.com/cauebraz/squeeze/internal/sanitize"
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

func mustReport(t *testing.T, input []byte, o options) report {
	t.Helper()
	r, err := squeeze(input, o)
	if err != nil {
		t.Fatal(err)
	}
	return r.Data.(report)
}

func status(name, postfix, end string) string {
	return name + strings.Repeat(".", 79-width(sanitize.Line(name))-len(postfix)-len(end)) + postfix + end
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
		"passed":             {status("trim trailing whitespace", "", "Passed") + "\n", true},
		"no files":           {status("check json", noFiles, "Skipped") + "\n", true},
		"color":              {status("check yaml", "", "") + "\x1b[41mFailed\x1b[m\n", true},
		"after log lines":    {"[INFO] Initializing environment for x.\n" + status("a", "", "Failed") + "\n", true},
		"interrupted":        {strings.Repeat(".", 73) + "x\n" + "hook" + strings.Repeat(".", 69) + "Interrupted (^C): KeyboardInterrupt: \n", true},
		"short dotted line":  {"check yaml.......Passed\n", false},
		"indented":           {" " + status("x", "", "Passed") + "\n", false},
		"tab separated":      {"build\tlint\t" + status("x", "", "Passed") + "\n", false},
		"other word":         {status("x", "", "Passes") + "\n", false},
		"two dots":           {strings.Repeat("x", 71) + "..Passed\n", false},
		"pytest progress":    {"tests/test_cart.py ..........F...                                  [100%]\n", false},
		"go test text":       {"ok  \texample.com/p\t0.1s\n", false},
		"error message only": {"[ERROR] Your pre-commit configuration is unstaged.\n", false},
		"empty":              {"", false},
	}
	for name, tt := range tests {
		if got := (Format{}).Detect([]byte(tt.input)); got != tt.want {
			t.Errorf("%s: Detect() = %v, want %v", name, got, tt.want)
		}
	}
}

func TestParseFailedRun(t *testing.T) {
	t.Parallel()
	run, err := parse(fixture(t, "failed"))
	if err != nil {
		t.Fatal(err)
	}
	want := []hook{
		{Name: "trim trailing whitespace", ID: "trailing-whitespace", Status: statusFailed, ExitCode: 1, Modified: true},
		{Name: "fix end of files", ID: "end-of-file-fixer", Status: statusFailed, ExitCode: 1, Modified: true},
		{Name: "check yaml", ID: "check-yaml", Status: statusFailed, ExitCode: 1},
		{Name: "check json", Status: statusSkipped, NoFiles: true},
		{Name: "check for added large files", Status: statusPassed},
		{Name: "expand tabs in markdown", ID: "expand-tabs", Status: statusFailed, Modified: true},
		{Name: "forbid TODO markers", ID: "no-todo", Status: statusFailed, ExitCode: 1},
		{Name: "go vet", Status: statusSkipped, NoFiles: true},
	}
	if diff := cmp.Diff(want, run.hooks, cmpopts.IgnoreUnexported(hook{})); diff != "" {
		t.Errorf("hooks (-want +got):\n%s", diff)
	}
	wantLines := [][]string{
		{"Fixing pre_commit_hooks/util.py", "Fixing pre_commit_hooks/removed.py"},
		{"Fixing pre_commit_hooks/util.py", "Fixing CHANGELOG.md"},
		{"while parsing a flow sequence", `  in ".pre-commit-hooks.yaml", line 214, column 11`, "expected ',' or ']', but got '<stream end>'", `  in ".pre-commit-hooks.yaml", line 215, column 1`},
		nil, nil, nil,
		{"73:# TODO: sort keys", "pre_commit_hooks/check_yaml.py: TODO markers are not allowed", "39:# TODO drop python 3.9 support", "pre_commit_hooks/check_json.py: TODO markers are not allowed"},
		nil,
	}
	for i, h := range run.hooks {
		if diff := cmp.Diff(wantLines[i], h.lines, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("%s output (-want +got):\n%s", h.Name, diff)
		}
	}
	if len(run.other) != 0 || len(run.errors) != 0 {
		t.Errorf("other %q, errors %q", run.other, run.errors)
	}
}

func TestColorAndPipedRunsReadTheSame(t *testing.T) {
	t.Parallel()
	plain := mustReport(t, fixture(t, "failed"), defaultOptions())
	color := mustReport(t, fixture(t, "color"), defaultOptions())
	if diff := cmp.Diff(plain, color, cmp.AllowUnexported(hookReport{}), cmpopts.IgnoreUnexported(hook{})); diff != "" {
		t.Errorf("color report differs (-plain +color):\n%s", diff)
	}
}

func TestSkipEnvAndVerbose(t *testing.T) {
	t.Parallel()
	run, err := parse(fixture(t, "skip-env"))
	if err != nil {
		t.Fatal(err)
	}
	if h := run.hooks[2]; h.Name != "check yaml" || h.Status != statusSkipped || h.NoFiles {
		t.Errorf("SKIP hook = %+v", h)
	}
	r := mustReport(t, fixture(t, "verbose"), options{All: true})
	var got []string
	for _, h := range r.Hooks {
		got = append(got, verdict(h.hook))
	}
	want := []string{
		"fail trailing-whitespace (trim trailing whitespace): exit code 1, files modified",
		"fail end-of-file-fixer (fix end of files): exit code 1, files modified",
		"fail check-yaml (check yaml): exit code 1",
		"skip check-json (check json): no files to check",
		"pass check-added-large-files (check for added large files)",
		"fail expand-tabs (expand tabs in markdown): files modified",
		"fail no-todo (forbid TODO markers): exit code 1",
		"skip go-vet (go vet): no files to check",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("verdicts (-want +got):\n%s", diff)
	}
	if r.Hooks[0].Duration != 0.07 || r.Modified != 3 || r.Failed != 5 {
		t.Errorf("duration %v, modified %d, failed %d", r.Hooks[0].Duration, r.Modified, r.Failed)
	}
}

func TestShowDiffOnFailure(t *testing.T) {
	t.Parallel()
	r := mustReport(t, fixture(t, "show-diff"), defaultOptions())
	want := []string{".pre-commit-hooks.yaml", "CHANGELOG.md", "README.md", "pre_commit_hooks/check_json.py", "pre_commit_hooks/check_yaml.py", "pre_commit_hooks/removed.py"}
	if diff := cmp.Diff(want, r.ChangedFiles); diff != "" || len(r.Other) != 0 || r.DiffOmitted == 0 {
		t.Errorf("other %q, diff omitted %d, changed files (-want +got):\n%s", r.Other, r.DiffOmitted, diff)
	}
}

func TestInterruptedHookIsAnError(t *testing.T) {
	t.Parallel()
	res, err := squeeze(fixture(t, "interrupted"), defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	r := res.Data.(report)
	if r.Unfinished != 1 || r.Hooks[1].Name != "integration tests" || !cmp.Equal(r.Errors, []string{"Interrupted (^C): KeyboardInterrupt:"}) {
		t.Errorf("unfinished %d, hooks %+v, errors %q", r.Unfinished, r.Hooks, r.Errors)
	}
	if !cmp.Equal(res.Conditions, []string{"error", "fail"}) {
		t.Errorf("conditions %v", res.Conditions)
	}
}

func TestLogLinesBecomeDiagnostics(t *testing.T) {
	t.Parallel()
	res, err := squeeze(fixture(t, "first-run"), defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	want := "warning: Unexpected key(s) present at root: minimum_pre_commit_versoin\n"
	if !strings.Contains(res.Text, want) || strings.Contains(res.Text, "[INFO]") || !strings.Contains(strings.Join(res.Conditions, ","), "warning") {
		t.Errorf("conditions %v, text:\n%s", res.Conditions, res.Text)
	}
	explicit, err := squeeze([]byte("[ERROR] Your pre-commit configuration is unstaged.\n`git add .pre-commit-config.yaml` to fix this.\n"), defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	wantText := "pre-commit: 0 hooks\nerror: Your pre-commit configuration is unstaged.\nother output:\n  `git add .pre-commit-config.yaml` to fix this.\n"
	if explicit.Text != wantText {
		t.Errorf("text:\n%s\nwant:\n%s", explicit.Text, wantText)
	}
	if _, err := squeeze([]byte("hello\nworld\n"), defaultOptions()); !errors.Is(err, errNoRun) {
		t.Errorf("Squeeze() error = %v, want %v", err, errNoRun)
	}
}

func TestNoHookWithID(t *testing.T) {
	t.Parallel()
	r := mustReport(t, []byte("No hook with id `ruff` in stage `pre-commit`\n"), defaultOptions())
	if !cmp.Equal(r.Errors, []string{"No hook with id `ruff` in stage `pre-commit`"}) {
		t.Errorf("errors %q", r.Errors)
	}
}

func TestOutOfRangeMetaLinesStayVisible(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("9", 400)
	input := strings.Join([]string{
		status("slow", "", "Failed"),
		"- hook id: slow",
		"- duration: " + huge + "s",
		"- exit code: " + huge,
		"",
	}, "\n")
	res, err := squeeze([]byte(input), defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := core.Envelope("precommit", len(input), res, nil); err != nil {
		t.Errorf("Envelope() error: %v", err)
	}
	want := []string{"- duration: " + huge + "s", "- exit code: " + huge}
	if diff := cmp.Diff(want, res.Data.(report).Other); diff != "" {
		t.Errorf("other (-want +got):\n%s", diff)
	}
}

func TestStatusLinesShareTheRunWidth(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		status("outer", "", "Failed"),
		"- hook id: outer",
		"- exit code: 1",
		"",
		"inner" + strings.Repeat(".", 80) + "Passed",
		"- hook id: inner",
		"",
		status("last", "", "Passed"),
	}, "\n")
	run, err := parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(run.hooks) != 2 || len(run.hooks[0].lines) != 2 {
		t.Errorf("hooks %+v", run.hooks)
	}
}

func TestWideNamesCountTwoColumns(t *testing.T) {
	t.Parallel()
	input := status("检查 yaml", "", "Failed") + "\n" + status("lint", "", "Passed") + "\n"
	run, err := parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if !(Format{}).Detect([]byte(input)) || len(run.hooks) != 2 || run.hooks[0].Name != "检查 yaml" {
		t.Errorf("hooks %+v", run.hooks)
	}
	for _, r := range []rune{'a', 'é', '检', '한', 'ｱ', '🙂', '𠀀', '＊'} {
		if got, want := width(string(r)), map[bool]int{false: 1, true: 2}[r > 0x1000 && r != 'ｱ']; got != want {
			t.Errorf("width(%q) = %d, want %d", r, got, want)
		}
	}
}

func TestCutPutsNoticesWhereLinesWereCut(t *testing.T) {
	t.Parallel()
	lines := []string{"a", "b", "c", "FAIL x - AssertionError", "  assert 1", "d", "FAIL x - AssertionError", "  assert 1", "e", "1 failed"}
	output := func(lines []string, limit int) ([]string, int) {
		r := build(parsed{hooks: []hook{{Status: statusFailed, lines: lines}}}, options{MaxLines: limit})
		return r.Hooks[0].Output, r.Hooks[0].LinesOmitted
	}
	got, omitted := output(lines, 2)
	want := []string{"... 3 lines cut (--max-lines 0)", "FAIL x - AssertionError", "  assert 1", "... 4 lines cut (--max-lines 0)", "1 failed"}
	if diff := cmp.Diff(want, got); diff != "" || omitted != 7 {
		t.Errorf("omitted %d, lines (-want +got):\n%s", omitted, diff)
	}
	plain := []string{"checking a", "checking b", "checking c", "checking d"}
	got, omitted = output(plain, 2)
	want = []string{"checking a", "checking b", "... 2 lines cut (--max-lines 0)"}
	if diff := cmp.Diff(want, got); diff != "" || omitted != 2 {
		t.Errorf("no error lines: omitted %d, lines (-want +got):\n%s", omitted, diff)
	}
	if got, omitted := output(lines, 0); len(got) != len(lines) || omitted != 0 {
		t.Errorf("--max-lines 0 cut %d lines", omitted)
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := strings.Join([]string{
		status("bad\x1b[2Jname\u0007", "", "Failed"),
		"- hook id: id\x1b]0;title\u0007",
		"- exit code: 1",
		"",
		"out\u0085put\x1b[31m",
		"",
		"[WARNING] w\u009b31m",
		"tail fail x",
	}, "\n")
	for _, o := range []options{defaultOptions(), {All: true}} {
		r, err := squeeze([]byte(input), o)
		if err != nil {
			t.Fatal(err)
		}
		formattest.CheckText(t, r.Text)
		for _, l := range strings.Split(r.Text, "\n") {
			if strings.HasPrefix(l, "fail x") {
				t.Errorf("forged line %q in:\n%s", l, r.Text)
			}
		}
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(status("a", "", "Failed") + "\n- hook id: a\n- exit code: 2\n\nx\n\n[ERROR] e\n" + status("b", noFiles, "Skipped") + "\n"))
	f.Fuzz(func(t *testing.T, input []byte) {
		formattest.CheckStable(t, func() (core.Result, error) { return squeeze(input, defaultOptions()) })
		run, err := parse(input)
		if err != nil {
			return
		}
		for _, o := range []options{defaultOptions(), {MaxLines: 1}, {All: true}} {
			r := build(run, o)
			res := core.Result{Text: text(r), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			if r.Passed+r.Failed+r.Skipped+r.Unfinished != len(run.hooks) || r.Total != len(run.hooks) {
				t.Fatalf("counts %+v for %d hooks", r, len(run.hooks))
			}
			for _, h := range run.hooks {
				if (h.Status == statusFailed || o.All) && !strings.Contains(res.Text, verdict(h)+"\n") {
					t.Fatalf("hook %q not printed:\n%s", verdict(h), res.Text)
				}
			}
			for _, e := range run.errors {
				if !strings.Contains(res.Text, "error: "+e+"\n") {
					t.Fatalf("error %q not printed:\n%s", e, res.Text)
				}
			}
			for _, w := range run.warnings {
				if !strings.Contains(res.Text, "warning: "+w+"\n") {
					t.Fatalf("warning %q not printed:\n%s", w, res.Text)
				}
			}
			if _, err := core.Envelope("precommit", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

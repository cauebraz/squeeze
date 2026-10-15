package precommit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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

func status(name, postfix, end string) string {
	return name + strings.Repeat(".", 79-width(core.Line(name))-len(postfix)-len(end)) + postfix + end
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
	run, err := Parse(fixture(t, "failed"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Hook{
		{Name: "trim trailing whitespace", ID: "trailing-whitespace", Status: Failed, ExitCode: 1, Modified: true},
		{Name: "fix end of files", ID: "end-of-file-fixer", Status: Failed, ExitCode: 1, Modified: true},
		{Name: "check yaml", ID: "check-yaml", Status: Failed, ExitCode: 1},
		{Name: "check json", Status: Skipped, NoFiles: true},
		{Name: "check for added large files", Status: Passed},
		{Name: "expand tabs in markdown", ID: "expand-tabs", Status: Failed, Modified: true},
		{Name: "forbid TODO markers", ID: "no-todo", Status: Failed, ExitCode: 1},
		{Name: "go vet", Status: Skipped, NoFiles: true},
	}
	if diff := cmp.Diff(want, run.Hooks, cmpopts.IgnoreUnexported(Hook{})); diff != "" {
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
	for i, h := range run.Hooks {
		if diff := cmp.Diff(wantLines[i], h.lines, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("%s output (-want +got):\n%s", h.Name, diff)
		}
	}
	if len(run.Other) != 0 || len(run.Errors) != 0 {
		t.Errorf("other %q, errors %q", run.Other, run.Errors)
	}
}

func TestColorAndPipedRunsReadTheSame(t *testing.T) {
	t.Parallel()
	plain := report(t, fixture(t, "failed"), DefaultOptions())
	color := report(t, fixture(t, "color"), DefaultOptions())
	if diff := cmp.Diff(plain, color, cmpopts.IgnoreUnexported(Hook{})); diff != "" {
		t.Errorf("color report differs (-plain +color):\n%s", diff)
	}
}

func TestSkipEnvAndVerbose(t *testing.T) {
	t.Parallel()
	run, err := Parse(fixture(t, "skip-env"))
	if err != nil {
		t.Fatal(err)
	}
	if h := run.Hooks[2]; h.Name != "check yaml" || h.Status != Skipped || h.NoFiles {
		t.Errorf("SKIP hook = %+v", h)
	}
	r := report(t, fixture(t, "verbose"), Options{All: true})
	var got []string
	for _, h := range r.Hooks {
		got = append(got, verdict(h.Hook))
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
	r := report(t, fixture(t, "show-diff"), DefaultOptions())
	want := []string{".pre-commit-hooks.yaml", "CHANGELOG.md", "README.md", "pre_commit_hooks/check_json.py", "pre_commit_hooks/check_yaml.py", "pre_commit_hooks/removed.py"}
	if diff := cmp.Diff(want, r.ChangedFiles); diff != "" || len(r.Other) != 0 || r.DiffOmitted == 0 {
		t.Errorf("other %q, diff omitted %d, changed files (-want +got):\n%s", r.Other, r.DiffOmitted, diff)
	}
}

func TestInterruptedHookIsAnError(t *testing.T) {
	t.Parallel()
	res, err := Squeeze(fixture(t, "interrupted"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	r := res.Data.(Report)
	if r.Unfinished != 1 || r.Hooks[1].Name != "integration tests" || !cmp.Equal(r.Errors, []string{"Interrupted (^C): KeyboardInterrupt:"}) {
		t.Errorf("unfinished %d, hooks %+v, errors %q", r.Unfinished, r.Hooks, r.Errors)
	}
	if !cmp.Equal(res.Conditions, []string{"error", "fail"}) {
		t.Errorf("conditions %v", res.Conditions)
	}
}

func TestLogLinesBecomeDiagnostics(t *testing.T) {
	t.Parallel()
	res, err := Squeeze(fixture(t, "first-run"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	want := "warning: Unexpected key(s) present at root: minimum_pre_commit_versoin\n"
	if !strings.Contains(res.Text, want) || strings.Contains(res.Text, "[INFO]") || !strings.Contains(strings.Join(res.Conditions, ","), "warning") {
		t.Errorf("conditions %v, text:\n%s", res.Conditions, res.Text)
	}
	explicit, err := Squeeze([]byte("[ERROR] Your pre-commit configuration is unstaged.\n`git add .pre-commit-config.yaml` to fix this.\n"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	wantText := "pre-commit: 0 hooks\nerror: Your pre-commit configuration is unstaged.\nother output:\n  `git add .pre-commit-config.yaml` to fix this.\n"
	if explicit.Text != wantText {
		t.Errorf("text:\n%s\nwant:\n%s", explicit.Text, wantText)
	}
	if _, err := Squeeze([]byte("hello\nworld\n"), DefaultOptions()); err != ErrNoRun {
		t.Errorf("Squeeze() error = %v, want %v", err, ErrNoRun)
	}
}

func TestNoHookWithID(t *testing.T) {
	t.Parallel()
	r := report(t, []byte("No hook with id `ruff` in stage `pre-commit`\n"), DefaultOptions())
	if !cmp.Equal(r.Errors, []string{"No hook with id `ruff` in stage `pre-commit`"}) {
		t.Errorf("errors %q", r.Errors)
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
	run, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Hooks) != 2 || len(run.Hooks[0].lines) != 2 {
		t.Errorf("hooks %+v", run.Hooks)
	}
}

func TestWideNamesCountTwoColumns(t *testing.T) {
	t.Parallel()
	input := status("检查 yaml", "", "Failed") + "\n" + status("lint", "", "Passed") + "\n"
	run, err := Parse([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if !(Format{}).Detect([]byte(input)) || len(run.Hooks) != 2 || run.Hooks[0].Name != "检查 yaml" {
		t.Errorf("hooks %+v", run.Hooks)
	}
	for _, r := range []rune{'a', 'é', '检', '한', 'ｱ', '🙂', '𠀀', '＊'} {
		if got, want := width(string(r)), map[bool]int{false: 1, true: 2}[r > 0x1000 && r != 'ｱ']; got != want {
			t.Errorf("width(%q) = %d, want %d", r, got, want)
		}
	}
}

func TestCutKeepsErrorLinesOrTheHead(t *testing.T) {
	t.Parallel()
	lines := []string{"a", "b", "c", "FAIL x - AssertionError", "  assert 1", "d", "FAIL x - AssertionError", "  assert 1", "e", "1 failed"}
	got, omitted := cut(lines, 2)
	want := []string{"FAIL x - AssertionError", "  assert 1", "1 failed", "... 7 lines cut (--max-lines 0)"}
	if diff := cmp.Diff(want, got); diff != "" || omitted != 7 {
		t.Errorf("omitted %d, lines (-want +got):\n%s", omitted, diff)
	}
	plain := []string{"checking a", "checking b", "checking c", "checking d"}
	got, omitted = cut(plain, 2)
	want = []string{"checking a", "checking b", "... 2 lines cut (--max-lines 0)"}
	if diff := cmp.Diff(want, got); diff != "" || omitted != 2 {
		t.Errorf("no error lines: omitted %d, lines (-want +got):\n%s", omitted, diff)
	}
	if got, omitted := cut(lines, 0); len(got) != len(lines) || omitted != 0 {
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
	for _, o := range []Options{DefaultOptions(), {All: true}} {
		r, err := Squeeze([]byte(input), o)
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
		run, err := Parse(input)
		if err != nil {
			return
		}
		for _, o := range []Options{DefaultOptions(), {MaxLines: 1}, {All: true}} {
			r := Build(run, o)
			res := core.Result{Text: Text(r, o), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			if r.Passed+r.Failed+r.Skipped+r.Unfinished != len(run.Hooks) || r.Total != len(run.Hooks) {
				t.Fatalf("counts %+v for %d hooks", r, len(run.Hooks))
			}
			for _, h := range run.Hooks {
				if (h.Status == Failed || o.All) && !strings.Contains(res.Text, verdict(h)+"\n") {
					t.Fatalf("hook %q not printed:\n%s", verdict(h), res.Text)
				}
			}
			for _, e := range run.Errors {
				if !strings.Contains(res.Text, "error: "+e+"\n") {
					t.Fatalf("error %q not printed:\n%s", e, res.Text)
				}
			}
			for _, w := range run.Warnings {
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

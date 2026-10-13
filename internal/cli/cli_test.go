package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
)

const formatsDir = "../formats"

var (
	formatList = strings.Join(DefaultRegistry().Names(), ", ")
	allOwners  = flagOwners("all")
)

func flagOwners(name string) string {
	var owners []string
	for _, f := range DefaultRegistry().Formats() {
		fs := flag.NewFlagSet(f.Name(), flag.ContinueOnError)
		f.Configure(fs)
		if fs.Lookup(name) != nil {
			owners = append(owners, f.Name())
		}
	}
	return strings.Join(owners, ", ")
}

func readFixture(t *testing.T, format, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(formatsDir, format, "testdata", name, "input.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type result struct {
	code   int
	stdout string
	stderr string
}

func runCLI(t *testing.T, input []byte, args ...string) result {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, bytes.NewReader(input), &stdout, &stderr)
	return result{code, stdout.String(), stderr.String()}
}

func TestRunAutoDetect(t *testing.T) {
	t.Parallel()
	r := runCLI(t, readFixture(t, "tfplan", "replace-forced"))
	if r.code != ExitOK || r.stderr != "" {
		t.Fatalf("code = %d, stderr = %q", r.code, r.stderr)
	}
	if !strings.HasPrefix(r.stdout, "terraform 1.9.5 plan: 1 to add, 0 to change, 2 to destroy, 1 to replace\n") {
		t.Errorf("stdout = %q", r.stdout)
	}
}

func TestRunFallback(t *testing.T) {
	t.Parallel()
	input := []byte("\x1b[32mok\x1b[0m 3 tests\nok 3 tests\nok 3 tests\nok 3 tests\n")
	want := "ok 3 tests\n... 3 more identical lines (--no-fold)\n"
	auto := runCLI(t, input)
	if auto.code != ExitOK || auto.stdout != want {
		t.Errorf("auto: code = %d, stdout = %q", auto.code, auto.stdout)
	}
	if auto.stderr != "squeeze: no format detected; used generic\n" {
		t.Errorf("auto: stderr = %q", auto.stderr)
	}
	named := runCLI(t, input, "generic")
	if named.code != ExitOK || named.stdout != want || named.stderr != "" {
		t.Errorf("named: code = %d, stdout = %q, stderr = %q", named.code, named.stdout, named.stderr)
	}
	if r := runCLI(t, input, "generic", "--fail-on", "explode"); r.code != ExitUsage || !strings.Contains(r.stderr, `generic has no condition "explode" (conditions: error, warning)`) {
		t.Errorf("named --fail-on: code = %d, stderr = %q", r.code, r.stderr)
	}
}

func TestDetectionPrefersPytestOverJSONStreams(t *testing.T) {
	t.Parallel()
	input := []byte("{\"Action\":\"run\",\"Package\":\"p\",\"Test\":\"T\"}\n" +
		"=========== test session starts ===========\nplatform linux -- Python 3.13.1, pytest-9.1.1, pluggy-1.6.0\ncollected 1 item\n\n" +
		"tests/t.py F\n=========== FAILURES ===========\n___ test_a ___\nE   assert 1 == 2\n=========== 1 failed in 0.01s ===========\n")
	f, err := DefaultRegistry().Detect(input)
	if err != nil || f.Name() != "pytest" {
		t.Errorf("Detect() = %v, %v, want pytest", f, err)
	}
	log := []byte("building image\nstep 1\nerror: layer missing\n12 files in 3.2s\ndone\n")
	if f, err := DefaultRegistry().Detect(log); err != nil || f.Name() != "generic" {
		t.Errorf("Detect(log) = %v, %v, want generic", f, err)
	}
}

func TestFailOnWorksOnGenericText(t *testing.T) {
	t.Parallel()
	input := []byte("step 1\nerror: layer missing\nwarning: deprecated base image\n")
	r := runCLI(t, input, "--fail-on", "error")
	if r.code != ExitCondition || !strings.Contains(r.stdout, "error: layer missing") || !strings.Contains(r.stderr, "--fail-on matched: error") {
		t.Errorf("auto: code = %d, stdout = %q, stderr = %q", r.code, r.stdout, r.stderr)
	}
	if r := runCLI(t, []byte("all good\n"), "generic", "--fail-on", "error,warning"); r.code != ExitOK {
		t.Errorf("clean text: code = %d, stderr = %q", r.code, r.stderr)
	}
}

func TestHelpGoesToStdout(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{"-h"}, {"tfplan", "--help"}, {"generic", "-h"}} {
		r := runCLI(t, nil, args...)
		if r.code != ExitOK || r.stderr != "" || !strings.HasPrefix(r.stdout, "usage: squeeze") {
			t.Errorf("%v: code = %d, stdout = %q, stderr = %q", args, r.code, r.stdout[:min(len(r.stdout), 40)], r.stderr)
		}
	}
	r := runCLI(t, nil, "tfplan", "-h")
	for _, want := range []string{"-max-attrs n\n", "-fail-on conditions\n", "listed below"} {
		if !strings.Contains(r.stdout, want) {
			t.Errorf("tfplan help lacks %q:\n%s", want, r.stdout)
		}
	}
}

func TestRunNamedFormatMatchesAutoDetect(t *testing.T) {
	t.Parallel()
	input := readFixture(t, "tfplan", "update-nested")
	auto := runCLI(t, input)
	named := runCLI(t, input, "tfplan")
	if auto.code != ExitOK || named.code != ExitOK || auto.stdout != named.stdout {
		t.Errorf("auto and named output differ:\n%s\n%s", auto.stdout, named.stdout)
	}
}

func TestRunFormatFlags(t *testing.T) {
	t.Parallel()
	r := runCLI(t, readFixture(t, "tfplan", "update-nested"), "tfplan", "--max-attrs", "1", "--output", "text")
	if r.code != ExitOK || !strings.Contains(r.stdout, "... 2 more attributes (--max-attrs 0)") {
		t.Errorf("code = %d, stdout = %q, stderr = %q", r.code, r.stdout, r.stderr)
	}
}

func TestRunJSONEnvelope(t *testing.T) {
	t.Parallel()
	input := readFixture(t, "tfplan", "update-nested")
	for _, args := range [][]string{{"--output", "json"}, {"tfplan", "--output=json"}} {
		r := runCLI(t, input, args...)
		if r.code != ExitOK {
			t.Fatalf("%v: code = %d, stderr = %q", args, r.code, r.stderr)
		}
		var doc struct {
			SchemaVersion int             `json:"schema_version"`
			Format        string          `json:"format"`
			InputBytes    int             `json:"input_bytes"`
			TextBytes     int             `json:"text_bytes"`
			Ratio         float64         `json:"ratio"`
			Data          json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal([]byte(r.stdout), &doc); err != nil {
			t.Fatalf("%v: invalid JSON: %v\n%s", args, err, r.stdout)
		}
		text := runCLI(t, input).stdout
		if doc.SchemaVersion != core.SchemaVersion || doc.Format != "tfplan" || doc.InputBytes != len(input) ||
			doc.TextBytes != len(text) || doc.Ratio != core.Ratio(len(text), len(input)) || len(doc.Data) < 2 {
			t.Errorf("%v: envelope = %+v", args, doc)
		}
	}
}

func TestRunErrors(t *testing.T) {
	t.Parallel()
	plan := readFixture(t, "tfplan", "create-only")
	tests := []struct {
		name   string
		input  []byte
		args   []string
		code   int
		stderr string
	}{
		{"unknown flag", plan, []string{"--bogus"}, ExitUsage, "flag provided but not defined"},
		{"format flag without format", plan, []string{"--max-attrs", "1"}, ExitUsage, "flag provided but not defined: -max-attrs"},
		{"unknown format", plan, []string{"cargotest"}, ExitUsage, "unknown format \"cargotest\" (formats: " + formatList + ")"},
		{"version under a format", plan, []string{"tfplan", "--version"}, ExitUsage, "flag provided but not defined: -version"},
		{"bad format flag value", plan, []string{"tfplan", "--max-attrs", "-1"}, ExitUsage, "must be 0 or more"},
		{"extra argument", plan, []string{"tfplan", "plan.json"}, ExitUsage, `unexpected argument "plan.json"`},
		{"bad output", plan, []string{"--output", "yaml"}, ExitUsage, "--output must be text or json"},
		{"empty input", nil, nil, ExitError, "empty input"},
		{"whitespace input", []byte(" \n\t"), []string{"tfplan"}, ExitError, "empty input"},
		{"binary input", []byte("\x7fELF\x00\x01"), nil, ExitUnknownFormat, "input matches no known format and is not text; name it: squeeze <format> (formats: " + formatList + ")"},
		{"named format on other input", []byte("collected 3 items\n"), []string{"tfplan"}, ExitError, "squeeze: tfplan: decode"},
		{"state file", []byte(`{"format_version":"1.0","values":{}}`), []string{"tfplan"}, ExitError, "terraform state, not a plan"},
		{"truncated plan", plan[:len(plan)/2], []string{"tfplan"}, ExitError, "decode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := runCLI(t, tt.input, tt.args...)
			if r.code != tt.code {
				t.Errorf("code = %d, want %d; stderr = %q", r.code, tt.code, r.stderr)
			}
			if r.stdout != "" {
				t.Errorf("stdout = %q, want empty on error", r.stdout)
			}
			if !strings.Contains(r.stderr, tt.stderr) {
				t.Errorf("stderr = %q, want it to contain %q", r.stderr, tt.stderr)
			}
		})
	}
}

func TestRunHelp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		args     []string
		contains []string
		absent   []string
	}{
		{
			args:     []string{"-h"},
			contains: []string{"usage: squeeze [flags] < input", "squeeze <format> [flags]", "tfplan       Terraform plan JSON", "-output", "-raw-dir", "-version"},
			absent:   []string{"-max-attrs"},
		},
		{
			args:     []string{"tfplan", "--help"},
			contains: []string{"usage: squeeze tfplan [flags] < input", "Terraform plan JSON", "-max-attrs", "-output", "-raw-dir"},
			absent:   []string{"-version"},
		},
	}
	for _, tt := range tests {
		r := runCLI(t, nil, tt.args...)
		if r.code != ExitOK || r.stderr != "" {
			t.Errorf("%v: code = %d, stderr = %q", tt.args, r.code, r.stderr)
		}
		for _, s := range tt.contains {
			if !strings.Contains(r.stdout, s) {
				t.Errorf("%v: help lacks %q:\n%s", tt.args, s, r.stdout)
			}
		}
		for _, s := range tt.absent {
			if strings.Contains(r.stdout, s) {
				t.Errorf("%v: help contains %q", tt.args, s)
			}
		}
	}
}

func TestRunVersion(t *testing.T) {
	t.Parallel()
	r := runCLI(t, nil, "--version")
	if r.code != ExitOK || r.stdout != "squeeze "+version()+"\n" || r.stderr != "" {
		t.Errorf("code = %d, stdout = %q, stderr = %q", r.code, r.stdout, r.stderr)
	}
	if version() == "" || strings.Contains(version(), "(devel)") {
		t.Errorf("version() = %q", version())
	}
}

func TestStdinGuard(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(file, readFixture(t, "tfplan", "create-only"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var stdout, stderr bytes.Buffer
	if code := Run(nil, Stdin(f), &stdout, &stderr); code != ExitOK {
		t.Errorf("regular file: code = %d, stderr = %q", code, stderr.String())
	}
	dev, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer dev.Close()
	stdout.Reset()
	stderr.Reset()
	code := Run(nil, Stdin(dev), &stdout, &stderr)
	if code != ExitError || !strings.Contains(stderr.String(), "stdin is a terminal") || stdout.Len() != 0 {
		t.Errorf("char device: code = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	if _, ok := Stdin(dev).(noPipe); !ok {
		t.Error("Stdin(char device) did not return the guard")
	}
	if r := runCLI(t, nil, "--version"); r.code != ExitOK {
		t.Errorf("--version must not read stdin: code = %d", r.code)
	}
}

func TestRunRawDir(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "nested", "raw")
	input := readFixture(t, "tfplan", "sensitive")
	r := runCLI(t, input, "--raw-dir", dir)
	if r.code != ExitOK {
		t.Fatalf("code = %d, stderr = %q", r.code, r.stderr)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("raw dir entries = %v, err = %v", entries, err)
	}
	path := filepath.Join(dir, entries[0].Name())
	if len(entries[0].Name()) != 12 {
		t.Errorf("raw file name = %q, want 12 hex characters", entries[0].Name())
	}
	if r.stderr != "squeeze: raw input saved to "+path+"\n" {
		t.Errorf("stderr = %q", r.stderr)
	}
	saved, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(saved, input) {
		t.Errorf("saved copy differs from input, err = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("raw file mode = %o, want 600", perm)
	}
	if again := runCLI(t, input, "tfplan", "--raw-dir", dir); again.stderr != r.stderr {
		t.Errorf("same input saved under a different path: %q vs %q", again.stderr, r.stderr)
	}
}

func TestRunRawDirSavedBeforeFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := runCLI(t, []byte("\x00binary"), "--raw-dir", dir)
	if r.code != ExitUnknownFormat {
		t.Fatalf("code = %d, want %d", r.code, ExitUnknownFormat)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("raw dir entries = %d, want 1", len(entries))
	}
}

func TestRunRawDirError(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r := runCLI(t, readFixture(t, "tfplan", "create-only"), "--raw-dir", file)
	if r.code != ExitError || !strings.Contains(r.stderr, "save raw input") || r.stdout != "" {
		t.Errorf("code = %d, stdout = %q, stderr = %q", r.code, r.stdout, r.stderr)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestRunIOErrors(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := Run(nil, failingReader{}, &stdout, &stderr); code != ExitError || !strings.Contains(stderr.String(), "read stdin: boom") {
		t.Errorf("read error: code = %d, stderr = %q", code, stderr.String())
	}
	stderr.Reset()
	input := readFixture(t, "tfplan", "create-only")
	if code := Run(nil, bytes.NewReader(input), failingWriter{}, &stderr); code != ExitError || !strings.Contains(stderr.String(), "write stdout: closed") {
		t.Errorf("write error: code = %d, stderr = %q", code, stderr.String())
	}
}

type unmarshalable struct{}

func (unmarshalable) MarshalJSON() ([]byte, error) { return nil, errors.New("nope") }

type stubFormat struct{}

func (stubFormat) Name() string         { return "stub" }
func (stubFormat) Description() string  { return "stub" }
func (stubFormat) Detect([]byte) bool   { return true }
func (stubFormat) Conditions() []string { return []string{"hit"} }
func (stubFormat) Configure(fs *flag.FlagSet) core.Squeezer {
	fs.Bool("shared", false, "")
	return func([]byte) (core.Result, error) {
		return core.Result{Text: "ok\n", Data: unmarshalable{}}, nil
	}
}

func TestRunRenderError(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	reg := core.NewRegistry(stubFormat{})
	if code := run([]string{"--output", "json"}, strings.NewReader("x"), &stdout, &stderr, reg); code != ExitError {
		t.Errorf("code = %d, want %d; stderr = %q", code, ExitError, stderr.String())
	}
	if !strings.Contains(stderr.String(), "render:") || stdout.Len() != 0 {
		t.Errorf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}

func TestEveryFixtureIsDetectedByExactlyItsOwnFormat(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	formats := reg.Formats()
	for _, owner := range formats {
		for _, c := range formattest.Cases(t, filepath.Join(formatsDir, owner.Name(), "testdata")) {
			head := core.Head(c.Input)
			if f, err := reg.Detect(c.Input); err != nil || f.Name() != owner.Name() {
				t.Errorf("%s/%s: registry detects %v, %v", owner.Name(), c.Name, f, err)
			}
			for _, f := range formats {
				if reg.IsFallback(f) && f.Name() != owner.Name() {
					continue
				}
				if got, want := f.Detect(head), f.Name() == owner.Name(); got != want {
					t.Errorf("%s/%s: %s.Detect() = %v, want %v", owner.Name(), c.Name, f.Name(), got, want)
				}
			}
		}
	}
}

func TestRunHints(t *testing.T) {
	t.Parallel()
	plan := readFixture(t, "tfplan", "create-only")
	file := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(file, plan, 0o600); err != nil {
		t.Fatal(err)
	}
	tests := map[string]struct {
		args []string
		want []string
	}{
		"format flag without format": {
			[]string{"--all"},
			[]string{"squeeze: flag provided but not defined: -all\n", "squeeze: -all is a flag of formats " + allOwners + ": squeeze <format> -all\n", "squeeze: run 'squeeze -h' for usage\n"},
		},
		"version under a format": {
			[]string{"tfplan", "-version"},
			[]string{"-version works without a format: squeeze -version", "squeeze: run 'squeeze tfplan -h' for usage"},
		},
		"misspelled format": {
			[]string{"tfpaln"},
			[]string{"unknown format \"tfpaln\" (formats: " + formatList + "); did you mean \"tfplan\"?"},
		},
		"file as format": {
			[]string{file},
			[]string{`is a file; squeeze reads stdin: squeeze < ` + file},
		},
		"file after format": {
			[]string{"tfplan", file},
			[]string{`is a file; squeeze reads stdin: squeeze tfplan < ` + file},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := runCLI(t, plan, tt.args...)
			if r.code != ExitUsage || r.stdout != "" {
				t.Errorf("code = %d, stdout = %q", r.code, r.stdout)
			}
			for _, w := range tt.want {
				if !strings.Contains(r.stderr, w) {
					t.Errorf("stderr lacks %q:\n%s", w, r.stderr)
				}
			}
			if strings.Contains(r.stderr, "Flags:") {
				t.Errorf("a usage error printed the full help:\n%s", r.stderr)
			}
		})
	}
}

func TestFlagHintNone(t *testing.T) {
	t.Parallel()
	reg := DefaultRegistry()
	if got := flagHint(errors.New("other error"), nil, reg); got != "" {
		t.Errorf("flagHint(other) = %q", got)
	}
	if got := flagHint(errors.New(undefinedFlag+"bogus"), nil, reg); got != "" {
		t.Errorf("flagHint(bogus) = %q", got)
	}
	tf, _ := reg.Lookup("tfplan")
	if got := flagHint(errors.New(undefinedFlag+"no-values"), tf, reg); got != "" {
		t.Errorf("flagHint(no-values) under tfplan = %q", got)
	}
	tj, _ := reg.Lookup("tfjson")
	if got := flagHint(errors.New(undefinedFlag+"no-values"), tj, reg); got != "-no-values is a flag of format tfplan: squeeze tfplan -no-values" {
		t.Errorf("flagHint(no-values) under tfjson = %q", got)
	}
	two := core.NewRegistry(stubFormat{}, namedStub{"other"})
	if got := flagHint(errors.New(undefinedFlag+"shared"), nil, two); got != "-shared is a flag of formats stub, other: squeeze <format> -shared" {
		t.Errorf("flagHint(shared) = %q", got)
	}
}

func TestClosest(t *testing.T) {
	t.Parallel()
	names := []string{"tfplan", "pytest", "gotest"}
	tests := map[string]string{"tfplan": "tfplan", "tfpaln": "tfplan", "pytset": "pytest", "go": "", "terraform": ""}
	for word, want := range tests {
		if got := closest(word, names); got != want {
			t.Errorf("closest(%q) = %q, want %q", word, got, want)
		}
	}
	for _, tt := range []struct {
		a, b string
		want int
	}{{"", "", 0}, {"a", "", 1}, {"", "ab", 2}, {"kitten", "sitting", 3}, {"tfplan", "tfpaln", 2}} {
		if got := levenshtein(tt.a, tt.b); got != tt.want {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

type namedStub struct{ name string }

func (n namedStub) Name() string        { return n.name }
func (n namedStub) Description() string { return n.name }
func (namedStub) Detect([]byte) bool    { return false }
func (namedStub) Conditions() []string  { return nil }
func (namedStub) Configure(fs *flag.FlagSet) core.Squeezer {
	fs.Bool("shared", false, "")
	return func([]byte) (core.Result, error) { return core.Result{}, nil }
}

func TestRunFailOn(t *testing.T) {
	t.Parallel()
	replace := readFixture(t, "tfplan", "replace-forced")
	noop := readFixture(t, "tfplan", "noop-only")
	text := runCLI(t, replace).stdout
	tests := map[string]struct {
		input  []byte
		args   []string
		code   int
		stderr string
	}{
		"matched":                 {replace, []string{"--fail-on", "destroy"}, ExitCondition, "squeeze: --fail-on matched: destroy\n"},
		"matched several":         {replace, []string{"--fail-on", "drift,replace", "--fail-on", "destroy"}, ExitCondition, "squeeze: --fail-on matched: destroy, replace\n"},
		"matched under a format":  {replace, []string{"tfplan", "--fail-on=replace"}, ExitCondition, "squeeze: --fail-on matched: replace\n"},
		"not matched":             {noop, []string{"--fail-on", "destroy,change"}, ExitOK, ""},
		"unknown condition auto":  {replace, []string{"--fail-on", "explode"}, ExitUsage, `--fail-on: tfplan has no condition "explode" (conditions: change, deferred`},
		"unknown condition named": {replace, []string{"tfplan", "--fail-on", "explode"}, ExitUsage, `--fail-on: tfplan has no condition "explode"`},
		"empty name":              {replace, []string{"--fail-on", "destroy,"}, ExitUsage, "empty condition name"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := runCLI(t, tt.input, tt.args...)
			if r.code != tt.code {
				t.Errorf("code = %d, want %d; stderr = %q", r.code, tt.code, r.stderr)
			}
			if !strings.Contains(r.stderr, tt.stderr) || (tt.stderr == "" && r.stderr != "") {
				t.Errorf("stderr = %q, want %q", r.stderr, tt.stderr)
			}
			switch tt.code {
			case ExitCondition:
				if r.stdout != text {
					t.Errorf("stdout must be the full output on exit 3, got %q", r.stdout)
				}
			case ExitUsage:
				if r.stdout != "" {
					t.Errorf("stdout = %q, want empty", r.stdout)
				}
			}
		})
	}
}

func TestRunFailOnJSON(t *testing.T) {
	t.Parallel()
	r := runCLI(t, readFixture(t, "tfplan", "replace-forced"), "--output", "json", "--fail-on", "replace,drift")
	if r.code != ExitCondition {
		t.Fatalf("code = %d, stderr = %q", r.code, r.stderr)
	}
	var doc struct {
		Conditions []string `json:"conditions"`
		FailedOn   []string `json:"failed_on"`
	}
	if err := json.Unmarshal([]byte(r.stdout), &doc); err != nil {
		t.Fatal(err)
	}
	if strings.Join(doc.Conditions, ",") != "change,destroy,replace" || strings.Join(doc.FailedOn, ",") != "replace" {
		t.Errorf("conditions = %v, failed_on = %v", doc.Conditions, doc.FailedOn)
	}
}

func TestFormatHelpListsConditions(t *testing.T) {
	t.Parallel()
	r := runCLI(t, nil, "tfplan", "-h")
	if !strings.Contains(r.stdout, "Conditions for --fail-on:\n  change, deferred, destroy, drift, error, fail, forget, import, incomplete, move, replace\n") {
		t.Errorf("help lacks the condition list:\n%s", r.stdout)
	}
	if r := runCLI(t, nil, "generic", "-h"); !strings.HasSuffix(r.stdout, "Conditions for --fail-on:\n  error, warning\n") {
		t.Errorf("generic help:\n%s", r.stdout)
	}
	root := runCLI(t, nil, "-h")
	if !strings.Contains(root.stdout, "-fail-on") || !strings.Contains(root.stdout, "squeeze <format> -h") {
		t.Errorf("root help lacks --fail-on:\n%s", root.stdout)
	}
}

func TestListFlag(t *testing.T) {
	t.Parallel()
	var values []string
	l := listFlag{&values}
	if err := l.Set("a, b,a"); err != nil || l.String() != "a,b" {
		t.Errorf("Set() = %v, String() = %q", err, l.String())
	}
	if (listFlag{}).String() != "" {
		t.Error("zero listFlag String() is not empty")
	}
}

func TestDocsFlagTablesMatchHelp(t *testing.T) {
	t.Parallel()
	common := map[string]bool{"output": true, "raw-dir": true, "fail-on": true}
	row := regexp.MustCompile("^\\| `--([a-z-]+)` \\| (`\\d+`|off) \\| (.*) \\|$")
	for _, f := range DefaultRegistry().Formats() {
		doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "formats", f.Name()+".md"))
		if err != nil {
			t.Fatal(err)
		}
		_, rest, ok := strings.Cut(string(doc), "## Flags\n")
		if !ok {
			t.Errorf("%s: docs page has no Flags section", f.Name())
			continue
		}
		section, _, _ := strings.Cut(rest, "\n## ")
		documented := map[string][2]string{}
		for _, line := range strings.Split(section, "\n") {
			if m := row.FindStringSubmatch(line); m != nil {
				documented[m[1]] = [2]string{m[2], m[3]}
			}
		}
		fs := flag.NewFlagSet(f.Name(), flag.ContinueOnError)
		f.Configure(fs)
		fs.VisitAll(func(fl *flag.Flag) {
			if common[fl.Name] {
				return
			}
			got, ok := documented[fl.Name]
			if !ok {
				t.Errorf("%s: flag --%s is not in the docs table", f.Name(), fl.Name)
				return
			}
			delete(documented, fl.Name)
			usage := fl.Usage
			wantDefault := "off"
			if i := strings.LastIndex(usage, " (default "); i >= 0 {
				wantDefault = "`" + strings.TrimSuffix(usage[i+len(" (default "):], ")") + "`"
				usage = strings.TrimPrefix(usage[:i], "`n` ")
			}
			if got[0] != wantDefault {
				t.Errorf("%s --%s: docs default %s, help %s", f.Name(), fl.Name, got[0], wantDefault)
			}
			if strings.ReplaceAll(got[1], "`", "") != usage {
				t.Errorf("%s --%s: docs say %q, help says %q", f.Name(), fl.Name, got[1], usage)
			}
		})
		for name := range documented {
			t.Errorf("%s: docs table lists --%s, which the format does not register", f.Name(), name)
		}
	}
}

func TestRunRawDirSkipsEmptyInput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := runCLI(t, []byte(" \n"), "--raw-dir", dir)
	if r.code != ExitError || strings.Contains(r.stderr, "raw input saved") {
		t.Errorf("code = %d, stderr = %q", r.code, r.stderr)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("raw dir entries = %d, want 0", len(entries))
	}
}

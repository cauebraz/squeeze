package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
)

const formatsDir = "../formats"

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
	if r.code != ExitOK || !strings.Contains(r.stdout, "... 2 more attributes (--max-attrs)") {
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
		{"unknown format", plan, []string{"pytest"}, ExitUsage, `unknown format "pytest" (formats: tfplan)`},
		{"version under a format", plan, []string{"tfplan", "--version"}, ExitUsage, "flag provided but not defined: -version"},
		{"bad format flag value", plan, []string{"tfplan", "--max-attrs", "-1"}, ExitUsage, "must be 0 or more"},
		{"extra argument", plan, []string{"tfplan", "plan.json"}, ExitUsage, `unexpected argument "plan.json"`},
		{"bad output", plan, []string{"--output", "yaml"}, ExitUsage, "--output must be text or json"},
		{"empty input", nil, nil, ExitError, "empty input"},
		{"whitespace input", []byte(" \n\t"), []string{"tfplan"}, ExitError, "empty input"},
		{"undetected input", []byte("collected 3 items\n"), nil, ExitUnknownFormat, "input matches no known format; name it: squeeze <format> (formats: tfplan)"},
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
		if r.code != ExitOK || r.stdout != "" {
			t.Errorf("%v: code = %d, stdout = %q", tt.args, r.code, r.stdout)
		}
		for _, s := range tt.contains {
			if !strings.Contains(r.stderr, s) {
				t.Errorf("%v: help lacks %q:\n%s", tt.args, s, r.stderr)
			}
		}
		for _, s := range tt.absent {
			if strings.Contains(r.stderr, s) {
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
	r := runCLI(t, []byte("not a known format\n"), "--raw-dir", dir)
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

func (stubFormat) Name() string        { return "stub" }
func (stubFormat) Description() string { return "stub" }
func (stubFormat) Detect([]byte) bool  { return true }
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
	formats := DefaultRegistry().Formats()
	for _, owner := range formats {
		for _, c := range formattest.Cases(t, filepath.Join(formatsDir, owner.Name(), "testdata")) {
			head := core.Head(c.Input)
			for _, f := range formats {
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
			[]string{"squeeze: flag provided but not defined: -all\n", "squeeze: -all is a flag of format tfplan: squeeze tfplan -all\n", "run 'squeeze -h' for usage\n"},
		},
		"version under a format": {
			[]string{"tfplan", "-version"},
			[]string{"-version works without a format: squeeze -version", "run 'squeeze tfplan -h' for usage"},
		},
		"misspelled format": {
			[]string{"tfpaln"},
			[]string{`unknown format "tfpaln" (formats: tfplan); did you mean "tfplan"?`},
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
	if got := flagHint(errors.New(undefinedFlag+"all"), tf, reg); got != "" {
		t.Errorf("flagHint(all) under tfplan = %q", got)
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
func (namedStub) Configure(fs *flag.FlagSet) core.Squeezer {
	fs.Bool("shared", false, "")
	return func([]byte) (core.Result, error) { return core.Result{}, nil }
}

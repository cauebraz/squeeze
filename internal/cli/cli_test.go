package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
	"github.com/google/go-cmp/cmp"
)

const formatsDir = "../formats"

var formatList = strings.Join(DefaultRegistry().Names(), ", ")

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

func TestRunAutoDetectPlanWithVariablesPastHead(t *testing.T) {
	t.Parallel()
	const secret = "hunter2-SECRET"
	vars := `"variables": {"db_password": {"value": "` + secret + `"}, "pad": {"value": "` + strings.Repeat("x", core.HeadSize) + `"},`
	input := bytes.Replace(readFixture(t, "tfplan", "replace-forced"), []byte(`"variables": {`), []byte(vars), 1)
	if bytes.Contains(core.Head(input), []byte(`"planned_values"`)) {
		t.Fatal("planned_values is inside the head; the test does not exercise detection past it")
	}
	r := runCLI(t, input)
	if r.code != ExitOK || r.stderr != "" {
		t.Fatalf("code = %d, stderr = %q", r.code, r.stderr)
	}
	if !strings.HasPrefix(r.stdout, "terraform 1.9.5 plan: ") {
		t.Errorf("stdout = %q", r.stdout)
	}
	if strings.Contains(r.stdout, secret) {
		t.Errorf("stdout holds the variable value: %q", r.stdout)
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
	if r := runCLI(t, input, "generic", "--fail-on", "explode"); r.code != ExitUsage || !strings.Contains(r.stderr, `generic has no condition "explode" (conditions: error, warning; fail: accepted, never present)`) {
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
		if r.code != ExitOK || r.stderr != "" || !strings.Contains(r.stdout, "\nUsage:\n  tool | squeeze ") {
			t.Errorf("%v: code = %d, stdout = %q, stderr = %q", args, r.code, r.stdout[:min(len(r.stdout), 40)], r.stderr)
		}
	}
	r := runCLI(t, nil, "tfplan", "-h")
	for _, want := range []string{"\ntfplan flags:\n", "  --max-attrs <n>      n attributes per resource listed, 0 for all (default 8)\n", "\nCommon flags:\n", "  --fail-on <list>     exit 3 when", "listed below"} {
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
		{"unknown flag", plan, []string{"--bogus"}, ExitUsage, "squeeze: unknown flag --bogus (squeeze -h lists flags)\n"},
		{"unknown format", plan, []string{"cargotest"}, ExitUsage, "unknown format \"cargotest\" (formats: " + formatList + ")"},
		{"version under a format", plan, []string{"tfplan", "--version"}, ExitUsage, "squeeze: --version is not a tfplan flag (without a format: squeeze --version)\n"},
		{"bad format flag value", plan, []string{"tfplan", "--max-attrs", "-1"}, ExitUsage, "must be 0 or more"},
		{"extra argument", plan, []string{"tfplan", "plan.json"}, ExitUsage, `unexpected argument "plan.json"`},
		{"bad output", plan, []string{"--output", "yaml"}, ExitUsage, "--output must be text or json"},
		{"unknown condition on empty input", nil, []string{"--fail-on", "bogus"}, ExitUsage, `--fail-on: no format has condition "bogus"`},
		{"bad detected-format flag value on empty input", nil, []string{"--max-attrs", "abc"}, ExitUsage, `invalid value "abc" for --max-attrs: not an integer`},
		{"binary input", []byte("\x7fELF\x00\x01"), nil, ExitUnknownFormat, "squeeze: input is binary (NUL byte at offset 4); squeeze reads text or JSON\n"},
		{"invalid UTF-8", []byte("caf\xe9 ok\n"), nil, ExitUnknownFormat, "squeeze: input is not text (invalid UTF-8 at byte 3); squeeze reads text or JSON, or name a format: squeeze <format> (squeeze -h lists formats)\n"},
		{"named format on other input", []byte("collected 3 items\n"), []string{"tfplan"}, ExitError, "squeeze: tfplan: input is not valid JSON (invalid character 'c' looking for beginning of value at byte 1); tfplan reads Terraform plan JSON from terraform show -json\n"},
		{"state file", []byte(`{"format_version":"1.0","values":{}}`), []string{"tfplan"}, ExitError, "terraform state, not a plan"},
		{"truncated plan", plan[:len(plan)/2], []string{"tfplan"}, ExitError, "squeeze: tfplan: input ends early (unexpected EOF); tfplan reads Terraform plan JSON"},
		{"plan of another shape", []byte("[1]"), []string{"tfplan"}, ExitError, "squeeze: tfplan: input is JSON of another shape (a JSON array at byte 1); tfplan reads"},
		{"broken XML", []byte("<testsuites><testsuite"), []string{"junit"}, ExitError, "squeeze: junit: input is not valid XML (line 1: unexpected EOF); junit reads JUnit XML"},
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

func TestEmptyInputSaysTheCommandPrintedNothing(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name  string
		input []byte
		args  []string
		want  string
	}{
		{"empty", nil, nil, "no output: the command printed nothing\n"},
		{"whitespace with a format", []byte(" \n\t"), []string{"tfplan"}, "no output: the command printed nothing\n"},
		{"fail-on", nil, []string{"--fail-on", "error,fail"}, "no output: the command printed nothing\n"},
		{"json", nil, []string{"--output", "json"}, `{"schema_version":1,"format":"","input_bytes":0,"text_bytes":39,"ratio":0,"conditions":[],"failed_on":[],"data":{}}` + "\n"},
		{"json with a format", []byte("\n"), []string{"tflint", "--output", "json"}, `{"schema_version":1,"format":"tflint","input_bytes":1,"text_bytes":39,"ratio":39,"conditions":[],"failed_on":[],"data":{}}` + "\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := runCLI(t, tt.input, tt.args...)
			if r.code != ExitOK || r.stdout != tt.want || r.stderr != "" {
				t.Errorf("code %d, stdout %q, stderr %q; want 0, %q, empty", r.code, r.stdout, r.stderr, tt.want)
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
			contains: []string{"squeeze compacts a tool's output for coding agents\n", "Usage:\n  tool | squeeze [format] [flags]\n", "Examples:\n  go test -json ./... | squeeze\n  terraform show -json plan.out | squeeze --fail-on destroy\n", "tfplan       Terraform plan JSON", "  --output text|json  output format: text, or json for the envelope with the format's data (default \"text\")\n", "  --raw-dir <dir>  ", "  --version  "},
			absent:   []string{"max-attrs", "\n  -output", "\n  -raw-dir"},
		},
		{
			args:     []string{"tfplan", "--help"},
			contains: []string{"Terraform plan JSON from terraform show -json\n\nUsage:\n  tool | squeeze tfplan [flags]\n", "--max-attrs", "--output", "--raw-dir"},
			absent:   []string{"version", "\n  -output", "\n  -max-attrs"},
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
	if want := "squeeze: --raw-dir " + file + ": cannot create: not a directory\n"; r.code != ExitError || r.stderr != want || r.stdout != "" {
		t.Errorf("code = %d, stdout = %q, stderr = %q; want stderr %q", r.code, r.stdout, r.stderr, want)
	}
}

func TestRunRawDirReplacesPartialFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	input := readFixture(t, "tfplan", "create-only")
	first := runCLI(t, input, "--raw-dir", dir)
	path := strings.TrimSuffix(strings.TrimPrefix(first.stderr, "squeeze: raw input saved to "), "\n")
	if err := os.WriteFile(path, input[:len(input)/2], 0o600); err != nil {
		t.Fatal(err)
	}
	if r := runCLI(t, input, "--raw-dir", dir); r.code != ExitOK || r.stderr != first.stderr {
		t.Fatalf("code = %d, stderr = %q", r.code, r.stderr)
	}
	saved, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(saved, input) {
		t.Errorf("partial file kept: %d of %d bytes, err = %v", len(saved), len(input), err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Errorf("raw dir entries = %v, err = %v; want only the saved file", entries, err)
	}
}

func TestRunRawDirReadOnly(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root writes to a read-only directory")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })
	r := runCLI(t, readFixture(t, "tfplan", "create-only"), "--raw-dir", dir)
	if want := "squeeze: --raw-dir " + dir + ": cannot save: permission denied\n"; r.code != ExitError || r.stdout != "" || r.stderr != want {
		t.Errorf("code = %d, stdout = %q, stderr = %q", r.code, r.stdout, r.stderr)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("raw dir entries = %v, want none", entries)
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

func TestFormatFlagsWorkWithDetection(t *testing.T) {
	t.Parallel()
	plan := readFixture(t, "tfplan", "create-only")
	named := runCLI(t, plan, "tfplan", "--max-attrs", "1")
	detected := runCLI(t, plan, "--max-attrs", "1", "--output", "json")
	namedJSON := runCLI(t, plan, "tfplan", "--max-attrs", "1", "--output", "json")
	if named.code != ExitOK || detected.code != ExitOK || detected.stdout != namedJSON.stdout {
		t.Errorf("detected code %d stdout %q; named JSON %q", detected.code, detected.stdout, namedJSON.stdout)
	}
	for _, tt := range []struct {
		args   []string
		stderr string
	}{
		{[]string{"--max-checks", "0"}, "squeeze: --max-checks is not a flag of the detected format tfplan (squeeze tfplan -h)\n"},
		{[]string{"--max-attrs", "-1"}, "squeeze: invalid value \"-1\" for --max-attrs: must be 0 or more\n"},
		{[]string{"--bogus"}, "squeeze: unknown flag --bogus (squeeze -h lists flags)\n"},
	} {
		r := runCLI(t, plan, tt.args...)
		if r.code != ExitUsage || r.stdout != "" || !strings.Contains(r.stderr, tt.stderr) {
			t.Errorf("%v: code %d, stdout %q, stderr %q", tt.args, r.code, r.stdout, r.stderr)
		}
	}
}

func TestFormatFlagNamesAgreeOnKind(t *testing.T) {
	t.Parallel()
	kinds := map[string]string{}
	for _, f := range DefaultRegistry().Formats() {
		fs := flag.NewFlagSet(f.Name(), flag.ContinueOnError)
		f.Configure(fs)
		fs.VisitAll(func(fl *flag.Flag) {
			kind := fmt.Sprintf("bool=%v", isBoolFlag(fl.Value))
			if prev, ok := kinds[fl.Name]; ok && prev != kind {
				t.Errorf("flag -%s is %s in %s but %s elsewhere", fl.Name, kind, f.Name(), prev)
			}
			kinds[fl.Name] = kind
		})
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
		"version under a format": {
			[]string{"tfplan", "-version"},
			[]string{"squeeze: --version is not a tfplan flag (without a format: squeeze --version)\n"},
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

func TestFlagErrors(t *testing.T) {
	t.Parallel()
	plan := readFixture(t, "tfplan", "create-only")
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--bogus"}, "squeeze: unknown flag --bogus (squeeze -h lists flags)\n"},
		{[]string{"--max-attrs"}, "squeeze: --max-attrs needs a value\n"},
		{[]string{"--max-attrs", "abc"}, "squeeze: invalid value \"abc\" for --max-attrs: not an integer\n"},
		{[]string{"--no-values=maybe"}, "squeeze: invalid boolean value \"maybe\" for --no-values: parse error\n"},
		{[]string{"---x"}, "squeeze: bad flag syntax: ---x (squeeze -h lists flags)\n"},
		{[]string{"generic", "---x"}, "squeeze: bad flag syntax: ---x (squeeze generic -h lists flags)\n"},
		{[]string{"generic", "--bogus"}, "squeeze: --bogus is not a generic flag (squeeze generic -h)\n"},
		{[]string{"generic", "--max-attrs", "3"}, "squeeze: --max-attrs is not a generic flag (format tfplan has it: squeeze tfplan --max-attrs)\n"},
		{[]string{"tfplan", "--fail-on", "destroy,"}, "squeeze: invalid value \"destroy,\" for --fail-on: empty condition name\n"},
	}
	for _, tt := range tests {
		r := runCLI(t, plan, tt.args...)
		if diff := cmp.Diff(result{ExitUsage, "", tt.want}, r, cmp.AllowUnexported(result{})); diff != "" {
			t.Errorf("%v (-want +got):\n%s", tt.args, diff)
		}
	}
	reg := core.NewRegistry(valueStub{"cur", false, ""}, stubFormat{}, namedStub{"other"})
	cur, _ := reg.Lookup("cur")
	if got, want := unknownFlag("shared", cur, reg), "--shared is not a cur flag (formats stub, other have it: squeeze <format> --shared)"; got != want {
		t.Errorf("unknownFlag(shared) = %q, want %q", got, want)
	}
}

func TestHelpAndVersionWords(t *testing.T) {
	t.Parallel()
	for _, tt := range [][2][]string{
		{{"help"}, {"-h"}},
		{{"help", "tfplan"}, {"tfplan", "-h"}},
		{{"version"}, {"--version"}},
	} {
		if diff := cmp.Diff(runCLI(t, nil, tt[1]...), runCLI(t, nil, tt[0]...), cmp.AllowUnexported(result{})); diff != "" {
			t.Errorf("%v differs from %v (-want +got):\n%s", tt[0], tt[1], diff)
		}
	}
	reg := core.NewRegistry(namedStub{"help"})
	if got := aliases([]string{"help"}, reg); !slices.Equal(got, []string{"help"}) {
		t.Errorf("aliases with a format named help = %v", got)
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
		"unknown condition auto":  {replace, []string{"--fail-on", "explode"}, ExitUsage, `--fail-on: no format has condition "explode"`},
		"other format condition":  {replace, []string{"--fail-on", "critical"}, ExitUsage, `--fail-on: tfplan has no condition "critical" (conditions: change, check, deferred`},
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
	if !strings.Contains(r.stdout, "Conditions for --fail-on:\n  change, check, deferred, destroy, drift, error, fail, forget, import, incomplete, move, replace, warning\n") {
		t.Errorf("help lacks the condition list:\n%s", r.stdout)
	}
	if r := runCLI(t, nil, "generic", "-h"); !strings.Contains(r.stdout, "Conditions for --fail-on:\n  error, warning\n  fail: accepted, never present for this format\n") {
		t.Errorf("generic help:\n%s", r.stdout)
	}
	root := runCLI(t, nil, "-h")
	if !strings.Contains(root.stdout, "--fail-on <list>") || !strings.Contains(root.stdout, "squeeze <format> -h") {
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

func TestSharedConditionsAreAcceptedEverywhere(t *testing.T) {
	t.Parallel()
	for _, f := range DefaultRegistry().Formats() {
		cases := formattest.Cases(t, filepath.Join(formatsDir, f.Name(), "testdata"))
		r := runCLI(t, cases[0].Input, f.Name(), "--fail-on", "error,fail,warning")
		if r.code != ExitOK && r.code != ExitCondition {
			t.Errorf("%s: code = %d, stderr = %q", f.Name(), r.code, r.stderr)
		}
	}
	r := runCLI(t, nil, "npmaudit", "-h")
	if !strings.Contains(r.stdout, "  critical, fail, fixable, high, info, low, moderate\n  error, warning: accepted, never present for this format\n") {
		t.Errorf("npmaudit help:\n%s", r.stdout)
	}
}

func TestREADMEFormatTableFollowsTheRegistry(t *testing.T) {
	t.Parallel()
	readme, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, line := range strings.Split(string(readme), "\n") {
		if strings.HasPrefix(line, "| `") {
			name, _, _ := strings.Cut(strings.TrimPrefix(line, "| `"), "`")
			if _, ok := DefaultRegistry().Lookup(name); ok {
				rows = append(rows, name)
			}
		}
	}
	if got := strings.Join(rows, ", "); got != formatList {
		t.Errorf("README rows %q, registry %q", got, formatList)
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
		_, conditions, ok := strings.Cut(string(doc), "## Conditions for `--fail-on`\n")
		if !ok {
			t.Errorf("%s: docs page has no conditions section", f.Name())
			continue
		}
		conditions, _, _ = strings.Cut(conditions, "\n## ")
		listed := map[string]bool{}
		for _, line := range strings.Split(conditions, "\n") {
			if m := condRow.FindStringSubmatch(line); m != nil {
				listed[m[1]] = true
			}
		}
		for _, name := range f.Conditions() {
			if !listed[name] {
				t.Errorf("%s: condition %q has no row in the docs", f.Name(), name)
			}
			delete(listed, name)
		}
		for name := range listed {
			t.Errorf("%s: docs list condition %q, which the format does not declare", f.Name(), name)
		}
	}
}

var condRow = regexp.MustCompile("^\\| `([a-z_]+)` \\| ")

func TestRunRawDirSkipsEmptyInput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := runCLI(t, []byte(" \n"), "--raw-dir", dir)
	if r.code != ExitOK || strings.Contains(r.stderr, "raw input saved") {
		t.Errorf("code = %d, stderr = %q", r.code, r.stderr)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("raw dir entries = %d, want 0", len(entries))
	}
}

type valueStub struct {
	name   string
	detect bool
	accept string
}

func (v valueStub) Name() string        { return v.name }
func (v valueStub) Description() string { return v.name }
func (v valueStub) Detect([]byte) bool  { return v.detect }
func (valueStub) Conditions() []string  { return nil }
func (v valueStub) Configure(fs *flag.FlagSet) core.Squeezer {
	fs.Func("level", "", func(s string) error {
		if s != v.accept {
			return errors.New("not " + v.accept)
		}
		return nil
	})
	return func([]byte) (core.Result, error) { return core.Result{Text: "ok\n"}, nil }
}

func TestDeferredFlagValueCheckedByEveryOwner(t *testing.T) {
	t.Parallel()
	reg := core.NewRegistry(valueStub{"picky", true, "low"}, valueStub{"lenient", false, "high"})
	for _, tt := range []struct {
		level, stderr string
		code          int
	}{
		{"low", "", ExitOK},
		{"high", "squeeze: invalid value \"high\" for --level: not low\n", ExitUsage},
		{"mid", "squeeze: invalid value \"mid\" for --level: not low\n", ExitUsage},
	} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"--level", tt.level}, strings.NewReader("x"), &stdout, &stderr, reg)
		if code != tt.code || stderr.String() != tt.stderr {
			t.Errorf("--level %s: code %d, stderr %q; want %d, %q", tt.level, code, stderr.String(), tt.code, tt.stderr)
		}
	}
}

func TestUndetectedWithoutFallback(t *testing.T) {
	t.Parallel()
	reg := core.NewRegistry(namedStub{"only"})
	if got, want := undetected([]byte("x"), reg), "input matches no known format; name it: squeeze <format> (formats: only)"; got != want {
		t.Errorf("undetected() = %q, want %q", got, want)
	}
	if got := invalidUTF8([]byte("ok")); got != 2 {
		t.Errorf("invalidUTF8(valid) = %d, want 2", got)
	}
}

func TestRawDirErrorReason(t *testing.T) {
	t.Parallel()
	link := &os.LinkError{Op: "rename", Old: "a", New: "b", Err: errors.New("busy")}
	if got, want := rawDirError("d", link), "--raw-dir d: cannot save: busy"; got != want {
		t.Errorf("rawDirError(link) = %q, want %q", got, want)
	}
	if got, want := rawDirError("d", errors.New("short write")), "--raw-dir d: cannot save: short write"; got != want {
		t.Errorf("rawDirError(other) = %q, want %q", got, want)
	}
}

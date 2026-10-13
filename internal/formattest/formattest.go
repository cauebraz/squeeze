package formattest

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
)

const RealDirEnv = "SQUEEZE_REAL_DIR"

var update = flag.Bool("update", false, "rewrite golden files")

type Case struct {
	Name  string
	Dir   string
	Input []byte
	Args  []string
}

func Cases(t testing.TB, dir string) []Case {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var cases []Case
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "real" {
			continue
		}
		caseDir := filepath.Join(dir, e.Name())
		inputs, err := filepath.Glob(filepath.Join(caseDir, "input.*"))
		if err != nil || len(inputs) != 1 {
			t.Fatalf("%s: want exactly one input.* file, found %v", caseDir, inputs)
		}
		input, err := os.ReadFile(inputs[0])
		if err != nil {
			t.Fatal(err)
		}
		c := Case{Name: e.Name(), Dir: caseDir, Input: input}
		if args, err := os.ReadFile(filepath.Join(caseDir, "args")); err == nil {
			c.Args = strings.Fields(string(args))
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		cases = append(cases, c)
	}
	if len(cases) == 0 {
		t.Fatalf("no cases in %s", dir)
	}
	return cases
}

func Inputs(t testing.TB, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, c := range Cases(t, dir) {
		if len(c.Args) == 0 {
			out[c.Name] = c.Input
		}
	}
	return out
}

func Run(t testing.TB, f core.Format, input []byte, args ...string) (core.Result, error) {
	t.Helper()
	fs := flag.NewFlagSet(f.Name(), flag.ContinueOnError)
	squeeze := f.Configure(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return squeeze(input)
}

func Contract(t *testing.T, f core.Format, dir string) {
	t.Helper()
	if f.Description() == "" {
		t.Errorf("%s: empty Description()", f.Name())
	}
	core.NewRegistry(f)
	for _, empty := range []string{"", " \n\t"} {
		if f.Detect([]byte(empty)) {
			t.Errorf("%s: Detect(%q) = true", f.Name(), empty)
		}
	}
	for _, c := range Cases(t, dir) {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			if !f.Detect(core.Head(c.Input)) {
				t.Errorf("Detect() = false")
			}
			first, err := Run(t, f, c.Input, c.Args...)
			if err != nil {
				t.Fatalf("squeeze error = %v", err)
			}
			second, err := Run(t, f, c.Input, c.Args...)
			if err != nil || second.Text != first.Text {
				t.Errorf("output differs between runs")
			}
			CheckText(t, first.Text)
			envelope, err := core.Envelope(f.Name(), len(c.Input), first)
			if err != nil {
				t.Fatalf("Envelope() error = %v", err)
			}
			if !json.Valid(envelope) {
				t.Errorf("Envelope() is not valid JSON: %s", envelope)
			}
			golden(t, filepath.Join(c.Dir, "want.txt"), []byte(first.Text))
			golden(t, filepath.Join(c.Dir, "want.json"), envelope)
		})
	}
}

func CheckText(t testing.TB, text string) {
	t.Helper()
	if text == "" || !strings.HasSuffix(text, "\n") {
		t.Errorf("text is empty or lacks a final newline: %q", text)
	}
	for i := 0; i < len(text); i++ {
		c := text[i]
		if (c < 0x20 && c != '\n') || c == 0x7f {
			t.Errorf("text contains control byte %#x at %d", c, i)
			return
		}
	}
}

func golden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v; run `just golden`", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from %s; run `just golden` and review the diff\ngot:\n%s\nwant:\n%s", path, got, want)
	}
}

func RealInputs(t testing.TB, f core.Format, defaultDir string) map[string][]byte {
	t.Helper()
	dir := defaultDir
	if env := os.Getenv(RealDirEnv); env != "" {
		dir = env
	}
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && p == dir {
				return filepath.SkipDir
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		input, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if f.Detect(core.Head(input)) {
			out[p] = input
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Skipf("no %s inputs in %s; save real tool output there or set %s", f.Name(), dir, RealDirEnv)
	}
	return out
}

func Real(t *testing.T, f core.Format, defaultDir string) {
	t.Helper()
	inputs := RealInputs(t, f, defaultDir)
	paths := make([]string, 0, len(inputs))
	for p := range inputs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		r, err := Run(t, f, inputs[p])
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		CheckText(t, r.Text)
		t.Logf("%s: %d -> %d bytes, ratio %.4f", p, len(inputs[p]), len(r.Text), core.Ratio(len(r.Text), len(inputs[p])))
	}
}

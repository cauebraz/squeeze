package formattest

import (
	"bytes"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

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
		if !e.IsDir() || e.Name() == "real" || e.Name() == "fuzz" {
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
	declared := f.Conditions()
	checkNames(t, f.Name()+": Conditions()", declared)
	for _, name := range declared {
		if !conditionName.MatchString(name) {
			t.Errorf("%s: condition %q must match %s", f.Name(), name, conditionName)
		}
	}
	for _, empty := range []string{"", " \n\t"} {
		if f.Detect([]byte(empty)) {
			t.Errorf("%s: Detect(%q) = true", f.Name(), empty)
		}
	}
	seen := map[string]bool{}
	var mu sync.Mutex
	cases := Cases(t, dir)
	t.Run("cases", func(t *testing.T) {
		for _, c := range cases {
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
				CheckNotices(t, first.Text)
				checkNames(t, "Result.Conditions", first.Conditions)
				mu.Lock()
				for _, c := range first.Conditions {
					seen[c] = true
				}
				mu.Unlock()
				for _, c := range first.Conditions {
					if !slices.Contains(declared, c) {
						t.Errorf("condition %q is not declared by Conditions()", c)
					}
				}
				envelope, err := core.Envelope(f.Name(), len(c.Input), first, nil)
				if err != nil {
					t.Fatalf("Envelope() error = %v", err)
				}
				var doc struct {
					Format string          `json:"format"`
					Data   json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal(envelope, &doc); err != nil || doc.Format != f.Name() || len(doc.Data) == 0 {
					t.Errorf("Envelope() does not round-trip: %v\n%s", err, envelope)
				}
				golden(t, filepath.Join(c.Dir, "want.txt"), []byte(first.Text))
				golden(t, filepath.Join(c.Dir, "want.json"), envelope)
			})
		}
	})
	for _, name := range declared {
		if !seen[name] {
			t.Errorf("%s: condition %q is declared but no case under %s produces it", f.Name(), name, dir)
		}
	}
}

var notice = regexp.MustCompile(`^\s*\.\.\. \d+ [^()]+ \((--[a-z-]+( 0)?|--output json|--raw-dir)\)$`)

func CheckNotices(t testing.TB, text string) {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "... ") && !notice.MatchString(line) {
			t.Errorf("cut notice does not follow \"... N <what> (<flag>)\": %q", line)
		}
	}
}

var conditionName = regexp.MustCompile(`^[a-z][a-z_]*$`)

func checkNames(t testing.TB, what string, names []string) {
	t.Helper()
	if !slices.IsSorted(names) {
		t.Errorf("%s is not sorted: %v", what, names)
	}
	if len(slices.Compact(slices.Clone(names))) != len(names) {
		t.Errorf("%s has duplicates: %v", what, names)
	}
}

func CheckText(t testing.TB, text string) {
	t.Helper()
	if text == "" || !strings.HasSuffix(text, "\n") {
		t.Errorf("text is empty or lacks a final newline: %q", text)
	}
	if !utf8.ValidString(text) {
		t.Errorf("text is not valid UTF-8")
	}
	for i, r := range text {
		if (r < 0x20 && r != '\n') || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			t.Errorf("text contains control character %#x at %d", r, i)
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

package formattest

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
)

var update = flag.Bool("update", false, "rewrite golden files")

func Fixtures(t testing.TB, dir string) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[e.Name()] = data
	}
	if len(out) == 0 {
		t.Fatalf("no fixtures in %s", dir)
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
	for name, input := range Fixtures(t, dir) {
		base := strings.TrimSuffix(name, filepath.Ext(name))
		t.Run(base, func(t *testing.T) {
			t.Parallel()
			if !f.Detect(core.Head(input)) {
				t.Errorf("Detect() = false")
			}
			first, err := Run(t, f, input)
			if err != nil {
				t.Fatalf("squeeze error = %v", err)
			}
			second, err := Run(t, f, input)
			if err != nil || second.Text != first.Text {
				t.Errorf("output differs between runs")
			}
			checkText(t, first.Text)
			envelope, err := core.Envelope(f.Name(), len(input), first)
			if err != nil {
				t.Fatalf("Envelope() error = %v", err)
			}
			if !json.Valid(envelope) {
				t.Errorf("Envelope() is not valid JSON: %s", envelope)
			}
			golden(t, filepath.Join(dir, "golden", base+".txt"), []byte(first.Text))
			golden(t, filepath.Join(dir, "golden", base+".json"), envelope)
		})
	}
}

func checkText(t *testing.T, text string) {
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
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
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

func Real(t *testing.T, f core.Format, dir string) {
	t.Helper()
	paths, _ := filepath.Glob(filepath.Join(dir, "*"))
	if len(paths) == 0 {
		t.Skipf("no inputs in %s; save real tool output there to run this test", dir)
	}
	for _, p := range paths {
		input, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		r, err := Run(t, f, input)
		if err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		t.Logf("%s: %d -> %d bytes, ratio %.4f", filepath.Base(p), len(input), len(r.Text), core.Ratio(len(r.Text), len(input)))
	}
}

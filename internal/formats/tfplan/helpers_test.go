package tfplan

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/formattest"
)

const fixtureDir = "testdata"

func decode(t testing.TB, s string) any {
	t.Helper()
	if s == "" {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(s)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}
	return v
}

func fixtures(t testing.TB) map[string][]byte {
	t.Helper()
	return formattest.Inputs(t, fixtureDir)
}

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name, "input.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func build(t testing.TB, input []byte, o Options) (Report, error) {
	t.Helper()
	p, err := Parse(input)
	if err != nil {
		return Report{}, err
	}
	return Build(p, o)
}

func mustBuild(t testing.TB, name string, o Options) Report {
	t.Helper()
	r, err := build(t, fixture(t, name), o)
	if err != nil {
		t.Fatalf("Build(%s) error = %v", name, err)
	}
	return r
}

func sensitiveStrings(t testing.TB, input []byte) []string {
	t.Helper()
	p, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	collect := func(c PlanChange) {
		out = append(out, sensitiveLeaves(c.Before, c.BeforeSensitive, false)...)
		out = append(out, sensitiveLeaves(c.After, c.AfterSensitive, false)...)
	}
	for _, list := range [][]PlanResource{p.ResourceChanges, p.ResourceDrift} {
		for _, rc := range list {
			collect(rc.Change)
		}
	}
	for _, d := range p.DeferredChanges {
		collect(d.ResourceChange.Change)
	}
	for _, c := range p.OutputChanges {
		collect(c)
	}
	return out
}

func sensitiveLeaves(value, marks any, inherited bool) []string {
	marked := inherited || isTrue(marks)
	switch v := value.(type) {
	case map[string]any:
		var out []string
		for k, child := range v {
			var childMarks any
			if m, ok := marks.(map[string]any); ok {
				childMarks = m[k]
			}
			out = append(out, sensitiveLeaves(child, childMarks, marked)...)
		}
		return out
	case []any:
		var out []string
		for i, child := range v {
			var childMarks any
			if l, ok := marks.([]any); ok && i < len(l) {
				childMarks = l[i]
			}
			out = append(out, sensitiveLeaves(child, childMarks, marked)...)
		}
		return out
	case string:
		if marked && len(v) >= 4 {
			return []string{v}
		}
	case json.Number:
		if marked && len(v) >= 4 {
			return []string{v.String()}
		}
	}
	return nil
}

func assertNoLeak(t testing.TB, name string, input []byte, outputs ...string) {
	t.Helper()
	for _, secret := range sensitiveStrings(t, input) {
		for _, out := range outputs {
			if strings.Contains(out, secret) {
				t.Errorf("%s leaks sensitive value %q", name, secret)
			}
		}
	}
}

func markedAt(marks any, p path) bool {
	if isTrue(marks) {
		return true
	}
	for _, s := range p {
		marks, _ = lookup(marks, s)
		if isTrue(marks) {
			return true
		}
	}
	return false
}

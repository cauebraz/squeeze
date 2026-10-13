package tfjson

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureDir = "testdata"

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name, "input.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func build(t testing.TB, name string, o Options) Report {
	t.Helper()
	lines, err := Parse(fixture(t, name))
	if err != nil {
		t.Fatalf("Parse(%s) error = %v", name, err)
	}
	return Build(lines, o)
}

func stream(lines ...string) []byte {
	return []byte(strings.Join(lines, "\n") + "\n")
}

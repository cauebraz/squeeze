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

func buildFixture(t testing.TB, name string, o options) report {
	t.Helper()
	lines, err := parse(fixture(t, name))
	if err != nil {
		t.Fatalf("Parse(%s) error = %v", name, err)
	}
	return build(lines, o)
}

func stream(lines ...string) []byte {
	return []byte(strings.Join(lines, "\n") + "\n")
}

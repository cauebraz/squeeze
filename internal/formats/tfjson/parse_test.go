package tfjson

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParse(t *testing.T) {
	t.Parallel()
	lines, err := parse(stream(
		"Acquiring state lock.",
		`{"@level":"info","@message":"Terraform 1.9.5","type":"version","terraform":"1.9.5"}`,
		"",
		`{"@level":"error","@message":"Error: x","type":"diagnostic","diagnostic":"not an object"}`,
		`{"not":"a stream line"}`,
		`{"@level":"info","type":5,"change":{"action":"create"}}`,
		`{"@level":"warn","type":"planned_change","change":{"action":1},"hook":{"elapsed_seconds":"x"}}`,
		`{"@level":"info","@message":"no newline at end","type":"log"}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, l := range lines {
		switch {
		case l.Type == "":
			got = append(got, "text:"+l.Text)
		case l.Malformed:
			got = append(got, "malformed:"+l.Type)
		default:
			got = append(got, l.Type)
		}
	}
	want := []string{
		"text:Acquiring state lock.", "version", "malformed:diagnostic", `text:{"not":"a stream line"}`,
		`text:{"@level":"info","type":5,"change":{"action":"create"}}`, "malformed:planned_change", "log",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("lines (-want +got):\n%s", diff)
	}
}

func TestParseWithoutStreamLines(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"", "\n\n", "plain text\n", `{"a":1}`, `[1,2]`} {
		if _, err := parse([]byte(input)); !errors.Is(err, errNoStream) {
			t.Errorf("Parse(%q) error = %v, want ErrNoStream", input, err)
		}
	}
}

func TestParseLongLine(t *testing.T) {
	t.Parallel()
	long := make([]byte, 0, 3<<20)
	long = append(long, `{"@level":"info","@message":"`...)
	for range 3 << 20 {
		long = append(long, 'x')
	}
	long = append(long, `","type":"log"}`...)
	lines, err := parse(long)
	if err != nil || len(lines) != 1 || lines[0].Type != "log" {
		t.Errorf("Parse(3 MiB line) = %d lines, %v", len(lines), err)
	}
}

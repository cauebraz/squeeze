package jsonl

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLines(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input string
		want  []string
	}{
		"empty":              {"", nil},
		"blank lines only":   {"\n \n\t\n", nil},
		"one line":           {`{"a":1}`, []string{`{"a":1}`}},
		"trailing newline":   {"{\"a\":1}\n", []string{`{"a":1}`}},
		"blank lines inside": {"{\"a\":1}\n\n  \n{\"b\":2}\n", []string{`{"a":1}`, `{"b":2}`}},
		"surrounding spaces": {"  {\"a\":1}  \r\n\t{\"b\":2}\r\n", []string{`{"a":1}`, `{"b":2}`}},
		"not json":           {"plain\ntext", []string{"plain", "text"}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var got []string
			for _, l := range Lines([]byte(tt.input)) {
				got = append(got, string(l))
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Lines() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	t.Parallel()
	isA := func(l []byte) bool { return bytes.HasPrefix(l, []byte("a")) }
	isComment := func(l []byte) bool { return bytes.HasPrefix(l, []byte("#")) }
	tests := map[string]struct {
		head string
		n    int
		skip func([]byte) bool
		want bool
	}{
		"empty":                        {"", 3, nil, false},
		"first line matches":           {"a1\nb\n", 3, nil, true},
		"match within n":               {"b\nc\na\n", 3, nil, true},
		"match past n":                 {"b\nc\nd\na\n", 3, nil, false},
		"blank lines do not count":     {"b\n\n \nc\na\n", 3, nil, true},
		"skipped lines do not count":   {"#x\n#y\nb\nc\na\n", 3, isComment, true},
		"skipped lines are not tested": {"ax\nb\n", 3, func(l []byte) bool { return bytes.HasPrefix(l, []byte("ax")) }, false},
		"comment counts without skip":  {"#x\nb\nc\na\n", 3, nil, false},
		"no match at all":              {"b\nc\n", 3, nil, false},
		"n of one":                     {"b\na\n", 1, nil, false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := Detect([]byte(tt.head), tt.n, tt.skip, isA); got != tt.want {
				t.Errorf("Detect(%q, %d) = %v, want %v", tt.head, tt.n, got, tt.want)
			}
		})
	}
}

func BenchmarkDetect(b *testing.B) {
	input := bytes.Repeat([]byte("{\"Action\":\"output\",\"Package\":\"p\",\"Output\":\"line of test output\"}\n"), 1<<20)
	match := func(l []byte) bool { return bytes.HasPrefix(l, []byte("[")) }
	b.ReportAllocs()
	for b.Loop() {
		Detect(input, 10, nil, match)
	}
}

func BenchmarkLines(b *testing.B) {
	input := bytes.Repeat([]byte("{\"Action\":\"output\",\"Package\":\"p\",\"Output\":\"line of test output\"}\n\n"), 1<<16)
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	for b.Loop() {
		Lines(input)
	}
}

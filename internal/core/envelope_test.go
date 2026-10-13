package core

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestEnvelope(t *testing.T) {
	t.Parallel()
	r := Result{Text: "0123456789", Data: map[string]int{"n": 1}}
	out, err := Envelope("demo", 100, r)
	if err != nil {
		t.Fatalf("Envelope() error = %v", err)
	}
	if !strings.HasSuffix(string(out), "}\n") || strings.Count(string(out), "\n") != 1 {
		t.Errorf("Envelope() is not one line ending in a newline: %q", out)
	}
	var doc struct {
		SchemaVersion int            `json:"schema_version"`
		Format        string         `json:"format"`
		InputBytes    int            `json:"input_bytes"`
		TextBytes     int            `json:"text_bytes"`
		Ratio         float64        `json:"ratio"`
		Data          map[string]int `json:"data"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if doc.SchemaVersion != SchemaVersion || doc.Format != "demo" || doc.InputBytes != 100 ||
		doc.TextBytes != 10 || doc.Ratio != 0.1 || doc.Data["n"] != 1 {
		t.Errorf("Envelope() = %+v", doc)
	}
}

func TestEnvelopeNilData(t *testing.T) {
	t.Parallel()
	out, err := Envelope("demo", 0, Result{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"data":null`) || !strings.Contains(string(out), `"ratio":0`) {
		t.Errorf("Envelope() = %s", out)
	}
}

type badJSON struct{}

func (badJSON) MarshalJSON() ([]byte, error) { return nil, errors.New("nope") }

func TestEnvelopeMarshalError(t *testing.T) {
	t.Parallel()
	if _, err := Envelope("demo", 1, Result{Data: badJSON{}}); err == nil {
		t.Fatal("Envelope() returned no error")
	}
}

func TestRatio(t *testing.T) {
	t.Parallel()
	tests := []struct {
		out, in int
		want    float64
	}{
		{0, 0, 0},
		{10, 0, 0},
		{10, -1, 0},
		{1, 3, 0.3333},
		{50, 100, 0.5},
		{200, 100, 2},
	}
	for _, tt := range tests {
		if got := Ratio(tt.out, tt.in); got != tt.want {
			t.Errorf("Ratio(%d, %d) = %v, want %v", tt.out, tt.in, got, tt.want)
		}
	}
}

func TestHead(t *testing.T) {
	t.Parallel()
	if got := Head([]byte("abc")); string(got) != "abc" {
		t.Errorf("Head(small) = %q", got)
	}
	if got := len(Head(make([]byte, HeadSize+10))); got != HeadSize {
		t.Errorf("len(Head(big)) = %d, want %d", got, HeadSize)
	}
}

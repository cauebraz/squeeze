package tfplan

import (
	"errors"
	"strings"
	"testing"
)

func TestParseAcceptsFormatVersions(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"1.0", "1.1", "1.2", "1"} {
		input := `{"format_version":"` + v + `","terraform_version":"1.9.5","planned_values":{},"resource_changes":[]}`
		p, err := Parse([]byte(input))
		if err != nil {
			t.Errorf("Parse(format_version %s) error = %v", v, err)
			continue
		}
		if p.FormatVersion != v || p.TerraformVersion != "1.9.5" {
			t.Errorf("Parse(format_version %s) = %+v", v, p)
		}
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantErr error
		wantMsg string
	}{
		{"empty", "", nil, "decode"},
		{"truncated", `{"format_version":"1.2","resource_changes":[`, nil, "decode"},
		{"array", `[1,2]`, nil, "decode"},
		{"string", `"plan"`, nil, "decode"},
		{"null", `null`, ErrMissingVersion, ""},
		{"no version", `{"resource_changes":[]}`, ErrMissingVersion, ""},
		{"wrong type", `{"format_version":1.2}`, nil, "decode"},
		{"major 2", `{"format_version":"2.0","planned_values":{}}`, nil, "unsupported format_version"},
		{"state", `{"format_version":"1.0","terraform_version":"1.9.5","values":{"root_module":{}}}`, ErrStateNotPlan, ""},
		{"bad actions type", `{"format_version":"1.2","resource_changes":[{"change":{"actions":"create"}}]}`, nil, "decode"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Parse([]byte(tt.input))
			if err == nil {
				t.Fatal("Parse() returned no error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("Parse() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantMsg != "" && !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("Parse() error = %v, want message containing %q", err, tt.wantMsg)
			}
		})
	}
}

func TestParseWithoutResourceChanges(t *testing.T) {
	t.Parallel()
	p, err := Parse([]byte(`{"format_version":"1.2","planned_values":{}}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(p.ResourceChanges) != 0 {
		t.Errorf("ResourceChanges = %v, want none", p.ResourceChanges)
	}
}

func TestParseKeepsNumbersExact(t *testing.T) {
	t.Parallel()
	input := `{"format_version":"1.2","planned_values":{},"resource_changes":[{"address":"a","change":{"actions":["update"],"before":{"n":12345678901234567890},"after":{"n":1.50}}}]}`
	p, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	before := p.ResourceChanges[0].Change.Before.(map[string]any)["n"]
	after := p.ResourceChanges[0].Change.After.(map[string]any)["n"]
	if s, _ := scalar(before); s != "12345678901234567890" {
		t.Errorf("before = %v, want exact integer", s)
	}
	if s, _ := scalar(after); s != "1.50" {
		t.Errorf("after = %v, want 1.50", s)
	}
}

func TestParseFixtures(t *testing.T) {
	t.Parallel()
	for name, data := range fixtures(t) {
		if _, err := Parse(data); err != nil {
			t.Errorf("Parse(%s) error = %v", name, err)
		}
	}
}

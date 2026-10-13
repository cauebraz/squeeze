package tfplan

import "testing"

func TestDetect(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"plan with planned_values", `{"format_version":"1.2","terraform_version":"1.9.5","planned_values":{}}`, true},
		{"plan with resource_changes", `{"format_version":"1.2","resource_changes":[]}`, true},
		{"leading whitespace", "\n\t  {\"format_version\":\"1.2\",\"planned_values\":{}}", true},
		{"state", `{"format_version":"1.0","terraform_version":"1.9.5","values":{}}`, false},
		{"json without version", `{"planned_values":{}}`, false},
		{"array", `[{"format_version":"1.2","planned_values":{}}]`, false},
		{"pytest", "============================= test session starts ==============================\nplatform darwin", false},
		{"empty", "", false},
		{"whitespace", "  \n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := (Format{}).Detect([]byte(tt.input)); got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDetectFixtures(t *testing.T) {
	t.Parallel()
	for name, data := range fixtures(t) {
		if !(Format{}).Detect(data) {
			t.Errorf("Detect(%s) = false", name)
		}
	}
}

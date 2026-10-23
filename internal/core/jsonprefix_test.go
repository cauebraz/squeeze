package core

import "testing"

func TestHasJSONPrefix(t *testing.T) {
	t.Parallel()
	prefix := []byte(`{"apiVersion":`)
	tests := map[string]bool{
		`{"apiVersion":"v1"}`:              true,
		"\n  {\n  \"apiVersion\" : \"v1\"": true,
		"{\t\"apiVersion\":":               true,
		`{"apiVersion"`:                    false,
		`{"kind":"Pod","apiVersion":"v1"}`: false,
		`{"apiVersions":"v1"}`:             false,
		"":                                 false,
		"   ":                              false,
		`[{"apiVersion":"v1"}]`:            false,
	}
	for head, want := range tests {
		if got := HasJSONPrefix([]byte(head), prefix); got != want {
			t.Errorf("HasJSONPrefix(%q) = %v, want %v", head, got, want)
		}
	}
	if !HasJSONPrefix([]byte("anything"), nil) {
		t.Error("an empty prefix does not match")
	}
}

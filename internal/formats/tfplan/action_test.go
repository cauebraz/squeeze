package tfplan

import (
	"strings"
	"testing"
)

func TestMapAction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		actions   []string
		importing bool
		want      Action
	}{
		{[]string{"no-op"}, false, Noop},
		{[]string{"create"}, false, Create},
		{[]string{"read"}, false, Read},
		{[]string{"update"}, false, Update},
		{[]string{"delete"}, false, Delete},
		{[]string{"delete", "create"}, false, Replace},
		{[]string{"create", "delete"}, false, ReplaceCBD},
		{[]string{"forget"}, false, Forget},
		{[]string{"no-op"}, true, Import},
		{[]string{"update"}, true, Update},
		{[]string{"create"}, true, Create},
	}
	for _, tt := range tests {
		got, err := mapAction(tt.actions, tt.importing)
		if err != nil || got != tt.want {
			t.Errorf("mapAction(%q, %v) = %v, %v; want %v", tt.actions, tt.importing, got, err, tt.want)
		}
	}
}

func TestMapActionUnsupported(t *testing.T) {
	t.Parallel()
	for _, actions := range [][]string{nil, {}, {"destroy"}, {"create", "forget"}, {"update", "update"}} {
		_, err := mapAction(actions, false)
		if err == nil {
			t.Errorf("mapAction(%q) returned no error", actions)
			continue
		}
		if !strings.Contains(err.Error(), "unsupported actions") {
			t.Errorf("mapAction(%q) error = %v", actions, err)
		}
	}
}

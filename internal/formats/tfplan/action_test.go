package tfplan

import (
	"strings"
	"testing"
)

func TestMapAction(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		actions   []string
		importing bool
		want      Action
	}{
		"no-op":                {[]string{"no-op"}, false, Noop},
		"create":               {[]string{"create"}, false, Create},
		"read":                 {[]string{"read"}, false, Read},
		"update":               {[]string{"update"}, false, Update},
		"delete":               {[]string{"delete"}, false, Delete},
		"delete then create":   {[]string{"delete", "create"}, false, Replace},
		"create then delete":   {[]string{"create", "delete"}, false, ReplaceCBD},
		"forget":               {[]string{"forget"}, false, Forget},
		"forget then create":   {[]string{"forget", "create"}, false, ForgetThenCreate},
		"create then forget":   {[]string{"create", "forget"}, false, CreateThenForget},
		"import without diff":  {[]string{"no-op"}, true, Import},
		"import with update":   {[]string{"update"}, true, Update},
		"import with creation": {[]string{"create"}, true, Create},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := mapAction(tt.actions, tt.importing)
			if err != nil || got != tt.want {
				t.Errorf("mapAction(%q, %v) = %v, %v; want %v", tt.actions, tt.importing, got, err, tt.want)
			}
		})
	}
}

func TestMapActionCoversEveryTerraformActionList(t *testing.T) {
	t.Parallel()
	terraform := [][]string{
		{"no-op"}, {"create"}, {"delete"}, {"update"}, {"create", "delete"}, {"read"},
		{"delete", "create"}, {"forget"}, {"forget", "create"}, {"create", "forget"},
	}
	for _, actions := range terraform {
		if _, err := mapAction(actions, false); err != nil {
			t.Errorf("mapAction(%q) error = %v", actions, err)
		}
	}
	if len(actionTable) != len(terraform) {
		t.Errorf("actionTable has %d entries, Terraform emits %d action lists", len(actionTable), len(terraform))
	}
}

func TestMapActionUnsupported(t *testing.T) {
	t.Parallel()
	for _, actions := range [][]string{nil, {}, {"destroy"}, {"update", "update"}, {"create", "create"}, {"Open"}} {
		_, err := mapAction(actions, false)
		if err == nil || !strings.Contains(err.Error(), "unsupported actions") {
			t.Errorf("mapAction(%q) error = %v", actions, err)
		}
	}
}

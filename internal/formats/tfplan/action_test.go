package tfplan

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMapAction(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		actions   []string
		importing bool
		want      action
	}{
		"no-op":                {[]string{"no-op"}, false, actionNoop},
		"create":               {[]string{"create"}, false, actionCreate},
		"read":                 {[]string{"read"}, false, actionRead},
		"update":               {[]string{"update"}, false, actionUpdate},
		"delete":               {[]string{"delete"}, false, actionDelete},
		"delete then create":   {[]string{"delete", "create"}, false, actionReplace},
		"create then delete":   {[]string{"create", "delete"}, false, actionReplaceCBD},
		"forget":               {[]string{"forget"}, false, actionForget},
		"forget then create":   {[]string{"forget", "create"}, false, actionForgetThenCreate},
		"create then forget":   {[]string{"create", "forget"}, false, actionCreateThenForget},
		"import without diff":  {[]string{"no-op"}, true, actionImport},
		"import with update":   {[]string{"update"}, true, actionUpdate},
		"import with creation": {[]string{"create"}, true, actionCreate},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got := mapAction(tt.actions, tt.importing); got != tt.want {
				t.Errorf("mapAction(%q, %v) = %v; want %v", tt.actions, tt.importing, got, tt.want)
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
		if got := mapAction(actions, false); got == actionUnknown {
			t.Errorf("mapAction(%q) = %v", actions, got)
		}
	}
	if len(actionTable) != len(terraform) {
		t.Errorf("actionTable has %d entries, Terraform emits %d action lists", len(actionTable), len(terraform))
	}
}

func TestMapActionUnsupported(t *testing.T) {
	t.Parallel()
	for _, actions := range [][]string{nil, {}, {"destroy"}, {"update", "update"}, {"create", "create"}, {"Open"}} {
		if got := mapAction(actions, false); got != actionUnknown {
			t.Errorf("mapAction(%q) = %v, want %v", actions, got, actionUnknown)
		}
	}
}

func TestRawActions(t *testing.T) {
	t.Parallel()
	if got := rawActions([]string{"create"}, actionCreate); got != nil {
		t.Errorf("rawActions(known) = %q, want nil", got)
	}
	got := rawActions([]string{"arch\nive", "delete"}, actionUnknown)
	if diff := cmp.Diff([]string{"arch ive", "delete"}, got); diff != "" {
		t.Errorf("rawActions() mismatch (-want +got):\n%s", diff)
	}
	if got := rawActions(nil, actionUnknown); got == nil || len(got) != 0 {
		t.Errorf("rawActions(nil) = %#v, want empty", got)
	}
}

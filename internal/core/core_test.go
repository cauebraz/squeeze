package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSharedConditionsCannotBeChanged(t *testing.T) {
	t.Parallel()
	SharedConditions()[0] = "changed"
	if diff := cmp.Diff([]string{"error", "fail", "warning"}, SharedConditions()); diff != "" {
		t.Errorf("SharedConditions() (-want +got):\n%s", diff)
	}
}

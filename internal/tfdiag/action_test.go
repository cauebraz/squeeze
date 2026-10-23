package tfdiag

import (
	"strings"
	"testing"
)

func TestActionInvocationItem(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a      ActionInvocation
		reason string
		want   string
	}{
		{ActionInvocation{Address: "action.a.n", Event: "AfterCreate", Resource: "aws_instance.web"}, "", "action.a.n (AfterCreate aws_instance.web)"},
		{ActionInvocation{Address: "action.a.o", Event: "BeforeDestroy"}, "", "action.a.o (BeforeDestroy)"},
		{ActionInvocation{Address: "action.a.m", Invoked: true}, "", "action.a.m (invoked)"},
		{ActionInvocation{Address: "action.a.p"}, "", "action.a.p"},
		{ActionInvocation{Address: "action.a.q", Event: "AfterUpdate", Resource: "aws_subnet.x"}, "deferred_prereq", "action.a.q (AfterUpdate aws_subnet.x; deferred_prereq)"},
		{ActionInvocation{Address: "action.a.r"}, "absent_prereq", "action.a.r (absent_prereq)"},
	}
	for _, tt := range tests {
		if got := tt.a.Item(tt.reason); got != tt.want {
			t.Errorf("Item(%q) = %q, want %q", tt.reason, got, tt.want)
		}
	}
}

func TestWriteActions(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	WriteActions(&b, nil)
	if b.Len() != 0 {
		t.Errorf("WriteActions(nil) wrote %q", b.String())
	}
	WriteActions(&b, []ActionInvocation{{Address: "a", Invoked: true}, {Address: "b"}})
	if got, want := b.String(), "actions: a (invoked), b\n"; got != want {
		t.Errorf("WriteActions() = %q, want %q", got, want)
	}
}

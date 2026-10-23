package tfplan

import (
	"encoding/json"
	"testing"
)

func TestActionTables(t *testing.T) {
	t.Parallel()
	ranks := map[int]action{}
	for _, a := range allActions {
		if a.String() == "" || a.symbol() == "" || (a.symbol() == "?") != (a == actionUnknown) {
			t.Errorf("action %d lacks a name or symbol", int(a))
		}
		if other, dup := ranks[a.rank()]; dup {
			t.Errorf("actions %s and %s share rank %d", a, other, a.rank())
		}
		ranks[a.rank()] = a
		text, err := a.MarshalText()
		if err != nil || string(text) != a.String() {
			t.Errorf("MarshalText(%s) = %q, %v", a, text, err)
		}
	}
	if len(allActions) != int(actionUnknown)+1 {
		t.Errorf("allActions has %d entries, want %d", len(allActions), int(actionUnknown)+1)
	}
}

func TestActionSymbolsMatchTerraform(t *testing.T) {
	t.Parallel()
	want := map[action]string{
		actionCreate: "+", actionDelete: "-", actionUpdate: "~", actionRead: "<=", actionReplace: "-/+", actionReplaceCBD: "+/-",
		actionForget: ".", actionForgetThenCreate: "./+", actionCreateThenForget: "+/.",
	}
	for a, sym := range want {
		if got := a.symbol(); got != sym {
			t.Errorf("%s.Symbol() = %q, want %q", a, got, sym)
		}
	}
}

func TestActionOrder(t *testing.T) {
	t.Parallel()
	order := []action{actionUnknown, actionDelete, actionReplace, actionReplaceCBD, actionForgetThenCreate, actionCreateThenForget, actionForget, actionUpdate, actionCreate, actionRead, actionImport, actionNoop}
	for i := 1; i < len(order); i++ {
		if order[i-1].rank() >= order[i].rank() {
			t.Errorf("%s must sort before %s", order[i-1], order[i])
		}
	}
}

func TestIsReplace(t *testing.T) {
	t.Parallel()
	for _, a := range allActions {
		want := a == actionReplace || a == actionReplaceCBD || a == actionForgetThenCreate || a == actionCreateThenForget
		if a.isReplace() != want {
			t.Errorf("%s.IsReplace() = %v", a, a.isReplace())
		}
	}
}

func TestInvalidEnums(t *testing.T) {
	t.Parallel()
	for _, a := range []action{-1, actionUnknown + 1} {
		if a.String() == "" || a.symbol() != "?" || a.rank() <= actionNoop.rank() {
			t.Errorf("invalid action %d: %q %q %d", int(a), a.String(), a.symbol(), a.rank())
		}
		if _, err := a.MarshalText(); err == nil {
			t.Errorf("MarshalText(%d) returned no error", int(a))
		}
	}
	for _, k := range []attrKind{-1, attrRemoved + 1} {
		if k.symbol() != "?" {
			t.Errorf("invalid kind %d symbol = %q", int(k), k.symbol())
		}
		if _, err := k.MarshalText(); err == nil {
			t.Errorf("MarshalText(kind %d) returned no error", int(k))
		}
	}
}

func TestAttrKind(t *testing.T) {
	t.Parallel()
	tests := map[attrKind][2]string{attrModified: {"update", "~"}, attrAdded: {"add", "+"}, attrRemoved: {"remove", "-"}}
	for k, want := range tests {
		text, err := k.MarshalText()
		if err != nil || string(text) != want[0] || k.symbol() != want[1] {
			t.Errorf("kind %d = %q %q, %v; want %v", int(k), text, k.symbol(), err, want)
		}
	}
}

func TestScalarJSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		v    any
		want string
	}{
		{nil, "null"}, {true, "true"}, {"a\"b", `"a\"b"`}, {json.Number("12345678901234567890"), "12345678901234567890"}, {1.5, "1.5"},
	}
	for _, tt := range tests {
		got, err := json.Marshal(scalar{Value: tt.v})
		if err != nil || string(got) != tt.want {
			t.Errorf("Marshal(%v) = %s, %v; want %s", tt.v, got, err, tt.want)
		}
	}
}

func TestSummaryChanged(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		s    summary
		want bool
	}{
		"empty":             {summary{}, false},
		"only side effects": {summary{Noop: 3, Drift: 1, Deferred: 1, Actions: 1, Outputs: 2, Replace: 1}, false},
		"import":            {summary{Import: 1}, true},
		"add":               {summary{Add: 1}, true},
		"change":            {summary{Change: 1}, true},
		"destroy":           {summary{Destroy: 1}, true},
		"forget":            {summary{Forget: 1}, true},
		"move":              {summary{Move: 1}, true},
		"read":              {summary{Read: 1}, true},
	}
	for name, tt := range tests {
		if got := tt.s.changed(); got != tt.want {
			t.Errorf("%s: Changed() = %v, want %v", name, got, tt.want)
		}
	}
}

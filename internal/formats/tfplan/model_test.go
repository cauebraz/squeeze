package tfplan

import (
	"encoding/json"
	"testing"
)

func TestActionTables(t *testing.T) {
	t.Parallel()
	ranks := map[int]Action{}
	for _, a := range allActions {
		if a.String() == "" || a.Symbol() == "" || a.Symbol() == "?" {
			t.Errorf("action %d lacks a name or symbol", int(a))
		}
		if other, dup := ranks[a.Rank()]; dup {
			t.Errorf("actions %s and %s share rank %d", a, other, a.Rank())
		}
		ranks[a.Rank()] = a
		text, err := a.MarshalText()
		if err != nil || string(text) != a.String() {
			t.Errorf("MarshalText(%s) = %q, %v", a, text, err)
		}
	}
	if len(allActions) != int(Import)+1 {
		t.Errorf("allActions has %d entries, want %d", len(allActions), int(Import)+1)
	}
}

func TestActionSymbolsMatchTerraform(t *testing.T) {
	t.Parallel()
	want := map[Action]string{
		Create: "+", Delete: "-", Update: "~", Read: "<=", Replace: "-/+", ReplaceCBD: "+/-",
		Forget: ".", ForgetThenCreate: "./+", CreateThenForget: "+/.",
	}
	for a, sym := range want {
		if got := a.Symbol(); got != sym {
			t.Errorf("%s.Symbol() = %q, want %q", a, got, sym)
		}
	}
}

func TestActionOrder(t *testing.T) {
	t.Parallel()
	order := []Action{Delete, Replace, ReplaceCBD, ForgetThenCreate, CreateThenForget, Forget, Update, Create, Read, Import, Noop}
	for i := 1; i < len(order); i++ {
		if order[i-1].Rank() >= order[i].Rank() {
			t.Errorf("%s must sort before %s", order[i-1], order[i])
		}
	}
}

func TestIsReplace(t *testing.T) {
	t.Parallel()
	for _, a := range allActions {
		want := a == Replace || a == ReplaceCBD || a == ForgetThenCreate || a == CreateThenForget
		if a.IsReplace() != want {
			t.Errorf("%s.IsReplace() = %v", a, a.IsReplace())
		}
	}
}

func TestInvalidEnums(t *testing.T) {
	t.Parallel()
	for _, a := range []Action{-1, Import + 1} {
		if a.String() == "" || a.Symbol() != "?" || a.Rank() <= Noop.Rank() {
			t.Errorf("invalid action %d: %q %q %d", int(a), a.String(), a.Symbol(), a.Rank())
		}
		if _, err := a.MarshalText(); err == nil {
			t.Errorf("MarshalText(%d) returned no error", int(a))
		}
	}
	for _, k := range []AttrKind{-1, Removed + 1} {
		if k.Symbol() != "?" {
			t.Errorf("invalid kind %d symbol = %q", int(k), k.Symbol())
		}
		if _, err := k.MarshalText(); err == nil {
			t.Errorf("MarshalText(kind %d) returned no error", int(k))
		}
	}
}

func TestAttrKind(t *testing.T) {
	t.Parallel()
	tests := map[AttrKind][2]string{Modified: {"update", "~"}, Added: {"add", "+"}, Removed: {"remove", "-"}}
	for k, want := range tests {
		text, err := k.MarshalText()
		if err != nil || string(text) != want[0] || k.Symbol() != want[1] {
			t.Errorf("kind %d = %q %q, %v; want %v", int(k), text, k.Symbol(), err, want)
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
		got, err := json.Marshal(Scalar{Value: tt.v})
		if err != nil || string(got) != tt.want {
			t.Errorf("Marshal(%v) = %s, %v; want %s", tt.v, got, err, tt.want)
		}
	}
}

func TestSummaryChanged(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		s    Summary
		want bool
	}{
		"empty":             {Summary{}, false},
		"only side effects": {Summary{Noop: 3, Drift: 1, Deferred: 1, Actions: 1, Outputs: 2, Replace: 1}, false},
		"import":            {Summary{Import: 1}, true},
		"add":               {Summary{Add: 1}, true},
		"change":            {Summary{Change: 1}, true},
		"destroy":           {Summary{Destroy: 1}, true},
		"forget":            {Summary{Forget: 1}, true},
		"move":              {Summary{Move: 1}, true},
		"read":              {Summary{Read: 1}, true},
	}
	for name, tt := range tests {
		if got := tt.s.Changed(); got != tt.want {
			t.Errorf("%s: Changed() = %v, want %v", name, got, tt.want)
		}
	}
}

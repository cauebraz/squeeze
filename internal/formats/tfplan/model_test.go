package tfplan

import "testing"

func TestActionNamesSymbolsRanks(t *testing.T) {
	t.Parallel()
	tests := []struct {
		action Action
		name   string
		symbol string
		rank   int
	}{
		{Delete, "delete", "-", 0},
		{Replace, "replace", "-/+", 1},
		{ReplaceCBD, "replace-create-before-destroy", "+/-", 2},
		{Forget, "forget", "-", 3},
		{Update, "update", "~", 4},
		{Create, "create", "+", 5},
		{Read, "read", "<=", 6},
		{Import, "import", ">>", 7},
		{Noop, "no-op", "=", 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.action.String(); got != tt.name {
				t.Errorf("String() = %q, want %q", got, tt.name)
			}
			if got := tt.action.Symbol(); got != tt.symbol {
				t.Errorf("Symbol() = %q, want %q", got, tt.symbol)
			}
			if got := tt.action.Rank(); got != tt.rank {
				t.Errorf("Rank() = %d, want %d", got, tt.rank)
			}
			text, err := tt.action.MarshalText()
			if err != nil || string(text) != tt.name {
				t.Errorf("MarshalText() = %q, %v; want %q, nil", text, err, tt.name)
			}
		})
	}
}

func TestInvalidAction(t *testing.T) {
	t.Parallel()
	for _, a := range []Action{-1, Forget + 1} {
		if got := a.String(); got == "" {
			t.Errorf("String() of %d is empty", int(a))
		}
		if got := a.Symbol(); got != "?" {
			t.Errorf("Symbol() of %d = %q, want ?", int(a), got)
		}
		if got := a.Rank(); got <= Noop.Rank() {
			t.Errorf("Rank() of %d = %d, want after no-op", int(a), got)
		}
		if _, err := a.MarshalText(); err == nil {
			t.Errorf("MarshalText() of %d returned no error", int(a))
		}
	}
}

func TestSummaryChanged(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		s    Summary
		want bool
	}{
		{"empty", Summary{}, false},
		{"noop drift outputs only", Summary{Noop: 3, Drift: 1, Outputs: 2}, false},
		{"add", Summary{Add: 1}, true},
		{"change", Summary{Change: 1}, true},
		{"destroy", Summary{Destroy: 1}, true},
		{"replace", Summary{Replace: 1}, true},
		{"read", Summary{Read: 1}, true},
		{"import", Summary{Import: 1}, true},
		{"forget", Summary{Forget: 1}, true},
		{"move", Summary{Move: 1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.s.Changed(); got != tt.want {
				t.Errorf("Changed() = %v, want %v", got, tt.want)
			}
		})
	}
}

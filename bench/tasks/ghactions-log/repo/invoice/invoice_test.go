package invoice

import (
	"errors"
	"testing"
	"time"

	"example.com/bench/billing/money"
)

func TestSubtotal(t *testing.T) {
	cases := []struct {
		name  string
		lines []Line
		want  money.Cents
	}{
		{"empty", nil, 0},
		{"one line", []Line{{"TEA-1", 3, 450}}, 1350},
		{"three lines", []Line{{"TEA-1", 2, 450}, {"MUG-2", 1, 1200}, {"POT-3", 1, 3999}}, 6099},
	}
	for _, c := range cases {
		if got := Subtotal(c.lines); got != c.want {
			t.Errorf("%s: Subtotal() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCompute(t *testing.T) {
	inv := Invoice{
		Number:   "INV-1042",
		Region:   "CA",
		Issued:   time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
		TermDays: 15,
		Lines:    []Line{{"TEA-1", 2, 450}, {"MUG-2", 1, 1200}, {"POT-3", 1, 3999}},
	}
	got, err := Compute(inv)
	if err != nil {
		t.Fatalf("Compute(%s): %v", inv.Number, err)
	}
	if got.Subtotal != 6099 || got.Tax != 442 || got.Total != 6541 {
		t.Errorf("Compute(%s) = subtotal %v, tax %v, total %v, want subtotal $60.99, tax $4.42, total $65.41", inv.Number, got.Subtotal, got.Tax, got.Total)
	}
	if d := got.Due.Format("2006-01-02"); d != "2026-10-19" {
		t.Errorf("Compute(%s) due %s, want 2026-10-19", inv.Number, d)
	}
}

func TestComputeUnknownRegion(t *testing.T) {
	_, err := Compute(Invoice{Region: "ZZ"})
	if !errors.Is(err, ErrRegion) {
		t.Errorf("Compute with region ZZ: err = %v, want ErrRegion", err)
	}
}

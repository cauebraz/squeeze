package tax

import (
	"testing"

	"example.com/bench/billing/money"
)

func TestCompute(t *testing.T) {
	cases := []struct {
		region   string
		subtotal money.Cents
		want     money.Cents
	}{
		{"NY", 10000, 887},
		{"NY", 1999, 177},
		{"NY", 5000, 444},
		{"CA", 4250, 308},
		{"CA", 100, 7},
		{"TX", 2999, 187},
		{"TX", 1000, 63},
		{"TX", 3000, 188},
		{"OR", 5000, 0},
	}
	for _, c := range cases {
		got, ok := For(c.region, c.subtotal)
		if !ok {
			t.Fatalf("For(%q) found no rate", c.region)
		}
		if got != c.want {
			t.Errorf("tax on %v in %s = %v, want %v", c.subtotal, c.region, got, c.want)
		}
	}
}

func TestUnknownRegion(t *testing.T) {
	if _, ok := For("ZZ", 100); ok {
		t.Errorf("For(%q) found a rate", "ZZ")
	}
}

package report

import (
	"strings"
	"testing"

	"example.com/bench/inventory/inventory"
)

func TestWrite(t *testing.T) {
	items := []inventory.Item{
		{SKU: "B2", Qty: 2, Price: 1.5},
		{SKU: "A1", Qty: 4, Price: 0.25},
	}
	var b strings.Builder
	if err := Write(&b, items); err != nil {
		t.Fatal(err)
	}
	want := "A1             1.00\nB2             3.00\nTOTAL          4.00\n"
	if b.String() != want {
		t.Fatalf("Write =\n%s\nwant\n%s", b.String(), want)
	}
}

func TestBanner(t *testing.T) {
	var b strings.Builder
	Banner(&b, "stock")
	if b.String() != "== stock ==\n\n" {
		t.Fatalf("Banner = %q", b.String())
	}
}

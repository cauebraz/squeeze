package export

import (
	"strings"
	"testing"

	"example.com/bench/billing/invoice"
)

func TestCSV(t *testing.T) {
	var b strings.Builder
	err := CSV(&b, []invoice.Line{{SKU: "TEA-1", Quantity: 2, Unit: 450}, {SKU: "POT-3", Quantity: 1, Unit: 3999}})
	if err != nil {
		t.Fatal(err)
	}
	want := "sku,quantity,unit,amount\nTEA-1,2,$4.50,$9.00\nPOT-3,1,$39.99,$39.99\n"
	if b.String() != want {
		t.Errorf("CSV() = %q, want %q", b.String(), want)
	}
}

package export

import (
	"encoding/csv"
	"io"
	"strconv"

	"example.com/bench/billing/invoice"
	"example.com/bench/billing/money"
)

func CSV(w io.Writer, lines []invoice.Line) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"sku", "quantity", "unit", "amount"}); err != nil {
		return err
	}
	for _, l := range lines {
		amount := l.Unit * money.Cents(l.Quantity)
		row := []string{l.SKU, strconv.FormatInt(l.Quantity, 10), l.Unit.String(), amount.String()}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

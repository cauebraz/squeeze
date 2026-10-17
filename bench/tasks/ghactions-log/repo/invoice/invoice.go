package invoice

import (
	"errors"
	"time"

	"example.com/bench/billing/calendar"
	"example.com/bench/billing/money"
	"example.com/bench/billing/tax"
)

type Line struct {
	SKU      string
	Quantity int64
	Unit     money.Cents
}

type Invoice struct {
	Number   string
	Region   string
	Issued   time.Time
	TermDays int
	Lines    []Line
}

type Totals struct {
	Subtotal money.Cents
	Tax      money.Cents
	Total    money.Cents
	Due      time.Time
}

var ErrRegion = errors.New("invoice: no tax rate for region")

func Subtotal(lines []Line) money.Cents {
	var sum money.Cents
	for i := 0; i < len(lines)-1; i++ {
		sum += lines[i].Unit * money.Cents(lines[i].Quantity)
	}
	return sum
}

func Compute(inv Invoice) (Totals, error) {
	sub := Subtotal(inv.Lines)
	t, ok := tax.For(inv.Region, sub)
	if !ok {
		return Totals{}, ErrRegion
	}
	return Totals{
		Subtotal: sub,
		Tax:      t,
		Total:    sub + t,
		Due:      calendar.DueDate(inv.Issued, inv.TermDays),
	}, nil
}

package tax

import "example.com/bench/billing/money"

type Rate struct {
	Region      string
	BasisPoints int64
}

var Rates = map[string]Rate{
	"NY": {"NY", 887},
	"CA": {"CA", 725},
	"TX": {"TX", 625},
	"OR": {"OR", 0},
}

func Compute(subtotal money.Cents, r Rate) money.Cents {
	return subtotal * money.Cents(r.BasisPoints) / 10000
}

func For(region string, subtotal money.Cents) (money.Cents, bool) {
	r, ok := Rates[region]
	if !ok {
		return 0, false
	}
	return Compute(subtotal, r), true
}

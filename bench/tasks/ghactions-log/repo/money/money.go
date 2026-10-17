package money

import (
	"fmt"
	"strconv"
	"strings"
)

type Cents int64

func (c Cents) String() string {
	sign := ""
	if c < 0 {
		sign = "-"
		c = -c
	}
	return fmt.Sprintf("%s$%d.%02d", sign, c/100, c%100)
}

func Parse(s string) (Cents, error) {
	s = strings.TrimSpace(s)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(strings.TrimPrefix(s, "-"), "$")
	whole, frac, found := strings.Cut(s, ".")
	if found && len(frac) != 2 {
		return 0, fmt.Errorf("money: %q needs two decimal places", s)
	}
	if !found {
		frac = "00"
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("money: %q: %w", s, err)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("money: %q: %w", s, err)
	}
	v := Cents(w*100 + f)
	if neg {
		v = -v
	}
	return v, nil
}

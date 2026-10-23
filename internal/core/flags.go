package core

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
)

func CountFlag(fs *flag.FlagSet, p *int, name, usage string) {
	fs.Func(name, fmt.Sprintf("`n` %s (default %d)", usage, *p), func(s string) error {
		n, err := strconv.Atoi(s)
		if err != nil {
			return errors.New("not an integer")
		}
		if n < 0 {
			return errors.New("must be 0 or more")
		}
		*p = n
		return nil
	})
}

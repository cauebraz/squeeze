package core

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"unicode/utf8"
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

func Plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func Limit[T any](items []T, max int) ([]T, int) {
	if max <= 0 || len(items) <= max {
		return items, 0
	}
	return items[:max], len(items) - max
}

func LimitTail[T any](items []T, max int) ([]T, int) {
	if max <= 0 || len(items) <= max {
		return items, 0
	}
	return items[len(items)-max:], len(items) - max
}

func Truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	if n == 0 {
		_, n = utf8.DecodeRuneInString(s)
	}
	return s[:n]
}

func Seconds(s float64) string {
	return strconv.FormatFloat(s, 'f', -1, 64) + "s"
}

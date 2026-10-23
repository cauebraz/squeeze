package core

import (
	"cmp"
	"errors"
	"flag"
	"fmt"
	"slices"
	"strconv"
	"strings"
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

func ToolLabel(tool, version string) string {
	if version == "" {
		return tool
	}
	return tool + " " + version
}

func Ellipsis(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return Truncate(s, n-3) + "..."
}

func Plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func Limit[T any](items []T, limit int) ([]T, int) {
	if limit <= 0 || len(items) <= limit {
		return items, 0
	}
	return items[:limit], len(items) - limit
}

func LimitTail[T any](items []T, limit int) ([]T, int) {
	if limit <= 0 || len(items) <= limit {
		return items, 0
	}
	return items[len(items)-limit:], len(items) - limit
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

type Count struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func SortedCounts(counts map[string]int) []Count {
	out := make([]Count, 0, len(counts))
	for name, n := range counts {
		out = append(out, Count{Name: name, Count: n})
	}
	slices.SortFunc(out, func(a, b Count) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(a.Name, b.Name))
	})
	return out
}

func TopCounts(counts []Count, limit int, one, many string) string {
	items := make([]string, 0, limit+1)
	for i, c := range counts {
		if i == limit {
			rest := len(counts) - limit
			items = append(items, fmt.Sprintf("%d other %s", rest, Plural(rest, one, many)))
			break
		}
		items = append(items, fmt.Sprintf("%s %d", c.Name, c.Count))
	}
	return strings.Join(items, ", ")
}

func Seconds(s float64) string {
	return strconv.FormatFloat(s, 'f', -1, 64) + "s"
}

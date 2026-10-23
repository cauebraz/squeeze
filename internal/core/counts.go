package core

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
)

type Part struct {
	N    int
	One  string
	Many string
}

func JoinCounts(parts ...Part) string {
	items := make([]string, 0, len(parts))
	for _, p := range parts {
		if p.N == 0 {
			continue
		}
		word := p.One
		if p.Many != "" {
			word = Plural(p.N, p.One, p.Many)
		}
		items = append(items, fmt.Sprintf("%d %s", p.N, word))
	}
	return strings.Join(items, ", ")
}

func UnknownNotice(counts map[string]int, one, many, field string) string {
	if len(counts) == 0 {
		return ""
	}
	total := 0
	items := make([]string, 0, len(counts))
	for _, k := range slices.Sorted(maps.Keys(counts)) {
		total += counts[k]
		name := NoticeName(k)
		if name == "" {
			name = "empty"
		}
		items = append(items, fmt.Sprintf("%s %d", name, counts[k]))
	}
	return fmt.Sprintf("... %d %s with an unknown %s: %s (--raw-dir)", total, Plural(total, one, many), field, strings.Join(items, ", "))
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

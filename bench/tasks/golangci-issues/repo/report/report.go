package report

import (
	"fmt"
	"io"
	"sort"

	"example.com/bench/inventory/inventory"
)

type Line struct {
	SKU   string
	Value float64
}

func Lines(items []inventory.Item) []Line {
	lines := make([]Line, 0, len(items))
	for _, it := range items {
		lines = append(lines, Line{SKU: it.SKU, Value: float64(it.Qty) * it.Price})
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].SKU < lines[j].SKU })
	return lines
}

func Write(w io.Writer, items []inventory.Item) error {
	total := 0.0
	for _, l := range Lines(items) {
		if _, err := fmt.Fprintf(w, "%-8s %10.2f\n", l.SKU, l.Value); err != nil {
			return err
		}
		total += l.Value
	}
	_, err := fmt.Fprintf(w, "%-8s %10.2f\n", "TOTAL", total)
	return err
}

func Banner(w io.Writer, title string) {
	fmt.Fprintf(w, "== %s ==\n", title)
	fmt.Fprintln(w)
}

func lowStock(items []inventory.Item, min int) []string {
	var out []string
	for _, it := range items {
		if it.Qty < min {
			out = append(out, it.SKU)
		}
	}
	return out
}

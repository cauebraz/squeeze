package core

import "fmt"

func TailLines(lines []string, max int, flag string) ([]string, int) {
	out := []string{}
	if max <= 0 || len(lines) <= max {
		return append(out, lines...), 0
	}
	keep := make([]bool, len(lines))
	for i, l := range lines {
		if i >= len(lines)-max {
			keep[i] = true
		}
		if ErrorLine(l) || WarningLine(l) {
			keep[i] = true
			if i+1 < len(lines) {
				keep[i+1] = true
			}
		}
	}
	omitted, gap := 0, 0
	for i, l := range lines {
		if !keep[i] {
			gap++
			omitted++
			continue
		}
		if gap > 0 {
			out = append(out, fmt.Sprintf("... %d %s cut (%s 0)", gap, Plural(gap, "line", "lines"), flag))
			gap = 0
		}
		out = append(out, l)
	}
	return out, omitted
}

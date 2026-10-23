package logscan

import "fmt"

func TailLines(lines []string, limit int, flag string) ([]string, int) {
	out := []string{}
	if limit <= 0 || len(lines) <= limit {
		return append(out, lines...), 0
	}
	keep := make([]bool, len(lines))
	for i, l := range lines {
		if i >= len(lines)-limit {
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
			out = append(out, gapNotice(gap, flag))
			gap = 0
		}
		out = append(out, l)
	}
	return out, omitted
}

func gapNotice(n int, flag string) string {
	if n == 1 {
		return fmt.Sprintf("... 1 line cut (%s 0)", flag)
	}
	return fmt.Sprintf("... %d lines cut (%s 0)", n, flag)
}

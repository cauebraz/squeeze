package generic

import "strings"

func text(r report) string {
	if len(r.out) == 0 {
		return "(no printable text)\n"
	}
	return strings.Join(r.out, "\n") + "\n"
}

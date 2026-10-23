package core

import (
	"iter"
	"strings"
)

var noticeName = strings.NewReplacer("(", "[", ")", "]")

func NoticeName(s string) string {
	return noticeName.Replace(s)
}

func Dedent(lines []string) []string {
	indent := -1
	for _, l := range lines {
		n := len(l) - len(strings.TrimLeft(l, " "))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	if indent <= 0 {
		return lines
	}
	for i, l := range lines {
		lines[i] = l[indent:]
	}
	return lines
}

func SplitLines(s string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for l := range strings.SplitSeq(s, "\n") {
			if !yield(strings.TrimRight(l, "\r")) {
				return
			}
		}
	}
}

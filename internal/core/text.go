package core

import (
	"iter"
	"strconv"
	"strings"
	"unicode/utf8"
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

func ToolLabel(tool, version string) string {
	if version == "" {
		return tool
	}
	return tool + " " + version
}

func Plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
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

func Ellipsis(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	if n <= 3 {
		return Truncate(s, n)
	}
	return Truncate(s, n-3) + "..."
}

func Seconds(s float64) string {
	return strconv.FormatFloat(s, 'f', -1, 64) + "s"
}

package sanitize

import (
	"strings"
	"unicode/utf8"
)

const (
	esc = 0x1b
	bel = 0x07
)

func Clean(s string) string {
	return strings.TrimSpace(scrub(s, " "))
}

func Line(s string) string {
	return strings.TrimRight(scrub(s, "    "), " ")
}

func NarrowLine(s string) string {
	return strings.TrimRight(scrub(s, " "), " ")
}

func scrub(s, tab string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == esc {
			i = skipEscape(s, i+1)
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		switch {
		case r == 0x9b:
			i = skipCSI(s, i)
		case r == 0x90 || r == 0x98 || r == 0x9d || r == 0x9e || r == 0x9f:
			i = skipString(s, i)
		case r == '\t':
			b.WriteString(tab)
		case r == '\n' || r == '\r' || r == 0x2028 || r == 0x2029:
			b.WriteByte(' ')
		case Invisible(r):
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func Invisible(r rune) bool {
	switch {
	case r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0):
		return true
	case r == 0xfeff || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069):
		return true
	}
	return false
}

func skipEscape(s string, j int) int {
	if j >= len(s) {
		return j
	}
	switch s[j] {
	case '[':
		return skipCSI(s, j+1)
	case ']', 'P', 'X', '^', '_':
		return skipString(s, j+1)
	}
	k := j
	for k < len(s) && s[k] >= 0x20 && s[k] <= 0x2f {
		k++
	}
	if k < len(s) && s[k] >= 0x30 && s[k] <= 0x7e {
		return k + 1
	}
	return j
}

func skipCSI(s string, k int) int {
	for ; k < len(s); k++ {
		switch c := s[k]; {
		case c >= 0x40 && c <= 0x7e:
			return k + 1
		case c < 0x20 || c > 0x7e:
			return k
		}
	}
	return k
}

func skipString(s string, k int) int {
	for ; k < len(s); k++ {
		switch {
		case s[k] == bel:
			return k + 1
		case s[k] == '\n':
			return k
		case s[k] == esc && k+1 < len(s) && s[k+1] == '\\':
			return k + 2
		case s[k] == 0xc2 && k+1 < len(s) && s[k+1] == 0x9c:
			return k + 2
		}
	}
	return k
}

package core

import (
	"strings"
	"unicode/utf8"
)

const esc = 0x1b

func Clean(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == esc {
			i = skipEscape(s, i)
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			b.WriteByte(' ')
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0):
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func skipEscape(s string, i int) int {
	j := i + 1
	if j >= len(s) {
		return j
	}
	switch s[j] {
	case '[':
		for k := j + 1; k < len(s); k++ {
			switch c := s[k]; {
			case c >= 0x40 && c <= 0x7e:
				return k + 1
			case c < 0x20 || c > 0x7e:
				return j + 1
			}
		}
		return j + 1
	case ']':
		for k := j + 1; k < len(s); k++ {
			switch {
			case s[k] == 0x07:
				return k + 1
			case s[k] == esc && k+1 < len(s) && s[k+1] == '\\':
				return k + 2
			}
		}
		return j + 1
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

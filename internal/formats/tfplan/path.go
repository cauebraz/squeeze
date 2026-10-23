package tfplan

import (
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type step struct {
	key     string
	index   int
	isIndex bool
	wild    bool
}

const maxMatchDepth = 32

type path []step

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

func (p path) String() string {
	return render(p, nil)
}

func render(p, before path) string {
	if len(p) == 0 {
		return "(resource)"
	}
	var b strings.Builder
	for i, s := range p {
		switch {
		case s.wild:
			b.WriteString("[*]")
		case s.isIndex:
			b.WriteByte('[')
			if i < len(before) && before[i].isIndex && before[i].index != s.index {
				b.WriteString(strconv.Itoa(before[i].index))
				b.WriteString("->")
			}
			b.WriteString(strconv.Itoa(s.index))
			b.WriteByte(']')
		case identifier.MatchString(s.key):
			if i > 0 {
				b.WriteByte('.')
			}
			b.WriteString(s.key)
		default:
			b.WriteByte('[')
			b.WriteString(strconv.Quote(s.key))
			b.WriteByte(']')
		}
	}
	return b.String()
}

func (p path) hasPrefix(prefix path) bool {
	if len(prefix) > len(p) {
		return false
	}
	for i := range prefix {
		if !p[i].matches(prefix[i]) {
			return false
		}
	}
	return true
}

func (s step) matches(o step) bool {
	return s == o || (s.wild && o.isIndex) || (o.wild && s.isIndex)
}

func (p path) lists() int {
	n := 0
	for _, s := range p {
		if s.isIndex {
			n++
		}
	}
	return n
}

func withoutIndices(p path) path {
	for i, s := range p {
		if s.isIndex {
			return p[:i]
		}
	}
	return p
}

func resolveWild(p path, values ...any) path {
	out := slices.Clone(p)
	for i, s := range out {
		if s.isIndex {
			for v := range values {
				values[v], _ = lookup(values[v], s)
			}
			continue
		}
		for _, v := range values {
			if _, isList := v.([]any); isList {
				out[i] = step{isIndex: true, wild: true}
				return out[:i+1]
			}
		}
		for v := range values {
			values[v], _ = lookup(values[v], s)
		}
	}
	return out
}

func (p path) overlaps(other path) bool {
	return p.hasPrefix(other) || other.hasPrefix(p)
}

func (p path) with(s step) path {
	out := make(path, len(p), len(p)+1)
	copy(out, p)
	return append(out, s)
}

func toPath(raw []any) (path, bool) {
	p := make(path, 0, len(raw))
	for _, r := range raw {
		switch v := r.(type) {
		case string:
			p = append(p, step{key: v})
		case json.Number:
			n, err := v.Int64()
			if err != nil {
				return nil, false
			}
			p = append(p, step{index: int(n), isIndex: true})
		case float64:
			if v != float64(int(v)) {
				return nil, false
			}
			p = append(p, step{index: int(v), isIndex: true})
		default:
			return nil, false
		}
	}
	return p, true
}

func index(i int) step {
	return step{index: i, isIndex: true}
}

func lookup(v any, s step) (any, bool) {
	if s.isIndex {
		if l, ok := v.([]any); ok && s.index >= 0 && s.index < len(l) {
			return l[s.index], true
		}
		return nil, false
	}
	if m, ok := v.(map[string]any); ok {
		val, present := m[s.key]
		return val, present
	}
	return nil, false
}

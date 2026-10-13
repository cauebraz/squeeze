package tfplan

import (
	"encoding/json"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const maxScalarBytes = 48

type step struct {
	key     string
	index   int
	isIndex bool
}

type path []step

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

func (p path) String() string {
	if len(p) == 0 {
		return "(resource)"
	}
	var b strings.Builder
	for i, s := range p {
		switch {
		case s.isIndex:
			b.WriteByte('[')
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
		if p[i] != prefix[i] {
			return false
		}
	}
	return true
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
			p = append(p, step{index: int(v), isIndex: true})
		default:
			return nil, false
		}
	}
	return p, true
}

type differ struct {
	values  bool
	replace []path
	out     []AttrChange
}

func diffAttrs(c PlanChange, values bool) ([]AttrChange, []string) {
	d := differ{values: values}
	var replacedBy []string
	for _, raw := range c.ReplacePaths {
		if p, ok := toPath(raw); ok {
			d.replace = append(d.replace, p)
			replacedBy = append(replacedBy, p.String())
		}
	}
	d.walk(nil, c.Before, c.After, c.AfterUnknown, c.BeforeSensitive, c.AfterSensitive)
	sort.SliceStable(d.out, func(i, j int) bool {
		return d.out[i].ForcesReplacement && !d.out[j].ForcesReplacement
	})
	return d.out, replacedBy
}

func (d *differ) walk(p path, before, after, unknown, beforeSens, afterSens any) {
	if isTrue(beforeSens) || isTrue(afterSens) {
		if isTrue(unknown) || !reflect.DeepEqual(before, after) {
			d.emit(p, AttrChange{Sensitive: true, Unknown: isTrue(unknown)})
		}
		return
	}
	if isTrue(unknown) {
		a := AttrChange{Unknown: true}
		if d.values && before != nil {
			if s, ok := scalar(before); ok {
				a.Before = s
			}
		}
		d.emit(p, a)
		return
	}
	bm, bIsMap := before.(map[string]any)
	am, aIsMap := after.(map[string]any)
	if bIsMap && aIsMap {
		for _, k := range unionKeys(bm, am, unknown) {
			s := step{key: k}
			d.walk(p.with(s), bm[k], am[k], child(unknown, s), child(beforeSens, s), child(afterSens, s))
		}
		return
	}
	bl, bIsList := before.([]any)
	al, aIsList := after.([]any)
	if bIsList && aIsList {
		n := max(len(bl), len(al))
		if ul, ok := unknown.([]any); ok {
			n = max(n, len(ul))
		}
		for i := range n {
			s := step{index: i, isIndex: true}
			d.walk(p.with(s), child(before, s), child(after, s), child(unknown, s), child(beforeSens, s), child(afterSens, s))
		}
		return
	}
	if reflect.DeepEqual(before, after) {
		return
	}
	a := AttrChange{}
	if d.values {
		bs, bok := scalar(before)
		as, aok := scalar(after)
		if bok && aok {
			a.Before, a.After = bs, as
		}
	}
	d.emit(p, a)
}

func (d *differ) emit(p path, a AttrChange) {
	a.Path = p.String()
	for _, r := range d.replace {
		if p.hasPrefix(r) || r.hasPrefix(p) {
			a.ForcesReplacement = true
			break
		}
	}
	d.out = append(d.out, a)
}

func isTrue(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

func child(v any, s step) any {
	if s.isIndex {
		if l, ok := v.([]any); ok && s.index >= 0 && s.index < len(l) {
			return l[s.index]
		}
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m[s.key]
	}
	return nil
}

func unionKeys(a, b map[string]any, extra any) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}
	if m, ok := extra.(map[string]any); ok {
		for k := range m {
			seen[k] = struct{}{}
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func scalar(v any) (string, bool) {
	var s string
	switch x := v.(type) {
	case nil:
		return "null", true
	case bool:
		return strconv.FormatBool(x), true
	case string:
		s = strconv.Quote(x)
	case json.Number:
		s = x.String()
	case float64:
		s = strconv.FormatFloat(x, 'g', -1, 64)
	default:
		return "", false
	}
	if len(s) > maxScalarBytes {
		return "", false
	}
	return s, true
}

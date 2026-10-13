package tfplan

import (
	"encoding/json"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

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

type node struct {
	before, after, unknown, beforeSens, afterSens any
	hasBefore, hasAfter                           bool
}

func rootNode(c PlanChange) node {
	return node{
		before: c.Before, after: c.After, unknown: c.AfterUnknown,
		beforeSens: c.BeforeSensitive, afterSens: c.AfterSensitive,
		hasBefore: true, hasAfter: true,
	}
}

func (n node) child(s step) node {
	before, hasBefore := lookup(n.before, s)
	after, hasAfter := lookup(n.after, s)
	unknown, _ := lookup(n.unknown, s)
	beforeSens, _ := lookup(n.beforeSens, s)
	afterSens, _ := lookup(n.afterSens, s)
	return node{
		before: before, after: after, unknown: unknown,
		beforeSens: beforeSens, afterSens: afterSens,
		hasBefore: n.hasBefore && hasBefore, hasAfter: n.hasAfter && hasAfter,
	}
}

func (n node) sensitive() bool {
	return isTrue(n.beforeSens) || isTrue(n.afterSens)
}

func (n node) kind() AttrKind {
	switch {
	case !n.hasBefore:
		return Added
	case !n.hasAfter && !isTrue(n.unknown):
		return Removed
	default:
		return Modified
	}
}

func (n node) childSteps() ([]step, bool) {
	bm, bIsMap := n.before.(map[string]any)
	am, aIsMap := n.after.(map[string]any)
	if bIsMap && aIsMap {
		keys := unionKeys(bm, am, n.unknown)
		steps := make([]step, 0, len(keys))
		for _, k := range keys {
			steps = append(steps, step{key: k})
		}
		return steps, true
	}
	bl, bIsList := n.before.([]any)
	al, aIsList := n.after.([]any)
	if bIsList && aIsList {
		size := max(len(bl), len(al))
		if ul, ok := n.unknown.([]any); ok {
			size = max(size, len(ul))
		}
		steps := make([]step, 0, size)
		for i := range size {
			steps = append(steps, step{index: i, isIndex: true})
		}
		return steps, true
	}
	return nil, false
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
	d.walk(nil, rootNode(c))
	sort.SliceStable(d.out, func(i, j int) bool {
		return d.out[i].ForcesReplacement && !d.out[j].ForcesReplacement
	})
	return d.out, replacedBy
}

func (d *differ) walk(p path, n node) {
	if !n.hasBefore && !n.hasAfter && !isTrue(n.unknown) {
		return
	}
	if n.sensitive() {
		if isTrue(n.unknown) || n.hasBefore != n.hasAfter || !reflect.DeepEqual(n.before, n.after) {
			d.emit(p, AttrChange{Kind: n.kind(), Sensitive: true, Unknown: isTrue(n.unknown)})
		}
		return
	}
	if isTrue(n.unknown) {
		a := AttrChange{Kind: n.kind(), Unknown: true}
		if n.hasBefore && n.before != nil {
			a.Before = d.scalar(n.before)
		}
		d.emit(p, a)
		return
	}
	if steps, ok := n.childSteps(); ok {
		for _, s := range steps {
			d.walk(p.with(s), n.child(s))
		}
		return
	}
	if n.hasBefore && n.hasAfter && reflect.DeepEqual(n.before, n.after) {
		return
	}
	a := AttrChange{Kind: n.kind()}
	switch a.Kind {
	case Added:
		a.After = d.scalar(n.after)
	case Removed:
		a.Before = d.scalar(n.before)
	case Modified:
		before, after := d.scalar(n.before), d.scalar(n.after)
		if before != nil && after != nil {
			a.Before, a.After = before, after
		}
	}
	d.emit(p, a)
}

func (d *differ) scalar(v any) *Scalar {
	if !d.values || !isScalar(v) {
		return nil
	}
	return &Scalar{Value: v}
}

func (d *differ) emit(p path, a AttrChange) {
	a.Path = p.String()
	a.at = p
	for _, r := range d.replace {
		if p.overlaps(r) {
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

func isScalar(v any) bool {
	switch v.(type) {
	case nil, bool, string, json.Number, float64:
		return true
	}
	return false
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

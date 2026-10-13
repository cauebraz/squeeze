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
	out := append(path(nil), p...)
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

func (n node) element(bi, ai int) node {
	before, hasBefore := lookupIndex(n.before, bi)
	after, hasAfter := lookupIndex(n.after, ai)
	unknown, _ := lookupIndex(n.unknown, ai)
	beforeSens, _ := lookupIndex(n.beforeSens, bi)
	afterSens, _ := lookupIndex(n.afterSens, ai)
	return node{
		before: before, after: after, unknown: unknown,
		beforeSens: beforeSens, afterSens: afterSens,
		hasBefore: n.hasBefore && hasBefore, hasAfter: n.hasAfter && hasAfter,
	}
}

func lookupIndex(v any, i int) (any, bool) {
	if i < 0 {
		return nil, false
	}
	return lookup(v, step{index: i, isIndex: true})
}

func elementKey(value, sens any) (string, bool) {
	if !hasTrue(sens) {
		sens = nil
	}
	key, err := json.Marshal([]any{value, sens})
	if err != nil {
		return "", false
	}
	return string(key), true
}

func hasTrue(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case []any:
		for _, e := range t {
			if hasTrue(e) {
				return true
			}
		}
	case map[string]any:
		for _, e := range t {
			if hasTrue(e) {
				return true
			}
		}
	}
	return false
}

type where struct {
	at, before path
}

func (w where) with(at, before step) where {
	return where{at: w.at.with(at), before: w.before.with(before)}
}

func index(i int) step {
	return step{index: i, isIndex: true}
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
			p = resolveWild(p, c.Before, c.After)
			d.replace = append(d.replace, p)
			replacedBy = append(replacedBy, p.String())
		}
	}
	d.walk(where{}, rootNode(c))
	sort.SliceStable(d.out, func(i, j int) bool {
		return d.out[i].ForcesReplacement && !d.out[j].ForcesReplacement
	})
	return d.out, replacedBy
}

func (d *differ) walk(w where, n node) {
	if !n.hasBefore && !n.hasAfter && !isTrue(n.unknown) {
		return
	}
	if n.sensitive() {
		if isTrue(n.unknown) || n.hasBefore != n.hasAfter || !reflect.DeepEqual(n.before, n.after) || isTrue(n.beforeSens) != isTrue(n.afterSens) {
			d.emit(w, AttrChange{Kind: n.kind(), Sensitive: true, Unknown: isTrue(n.unknown)})
		}
		return
	}
	if isTrue(n.unknown) {
		a := AttrChange{Kind: n.kind(), Unknown: true}
		if n.hasBefore && n.before != nil {
			a.Before = d.scalar(n.before)
		}
		d.emit(w, a)
		return
	}
	if bm, ok := n.before.(map[string]any); ok {
		if am, ok := n.after.(map[string]any); ok {
			for _, k := range unionKeys(bm, am, n.unknown) {
				s := step{key: k}
				d.walk(w.with(s, s), n.child(s))
			}
			return
		}
	}
	if bl, ok := n.before.([]any); ok {
		if al, ok := n.after.([]any); ok {
			d.walkList(w, n, bl, al)
			return
		}
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
	d.emit(w, a)
}

func (d *differ) walkList(w where, n node, before, after []any) {
	afterLen := len(after)
	if ul, ok := n.unknown.([]any); ok {
		afterLen = max(afterLen, len(ul))
	}
	if w.at.lists() >= maxMatchDepth {
		for i := range max(len(before), afterLen) {
			d.walk(w.with(index(i), index(i)), n.element(i, i))
		}
		return
	}
	free := map[string][]int{}
	for j := range afterLen {
		e := n.element(-1, j)
		if !e.hasAfter || hasTrue(e.unknown) {
			continue
		}
		if key, ok := elementKey(e.after, e.afterSens); ok {
			free[key] = append(free[key], j)
		}
	}
	matched := make([]bool, afterLen)
	var removed, added []int
	reordered := false
	last := -1
	for i := range before {
		e := n.element(i, -1)
		key, ok := elementKey(e.before, e.beforeSens)
		if q := free[key]; ok && len(q) > 0 {
			free[key] = q[1:]
			matched[q[0]] = true
			if q[0] < last {
				reordered = true
			}
			last = q[0]
			continue
		}
		removed = append(removed, i)
	}
	for j := range afterLen {
		if !matched[j] {
			added = append(added, j)
		}
	}
	if reordered {
		d.emit(w, AttrChange{Kind: Modified, Reordered: true})
	}
	paired := min(len(removed), len(added))
	for k := range paired {
		bi, ai := removed[k], added[k]
		_, bIsMap := n.element(bi, -1).before.(map[string]any)
		_, aIsMap := n.element(-1, ai).after.(map[string]any)
		if bi == ai || (bIsMap && aIsMap) {
			d.walk(w.with(index(ai), index(bi)), n.element(bi, ai))
			continue
		}
		d.walk(w.with(index(bi), index(bi)), n.element(bi, -1))
		d.walk(w.with(index(ai), index(ai)), n.element(-1, ai))
	}
	for _, bi := range removed[paired:] {
		d.walk(w.with(index(bi), index(bi)), n.element(bi, -1))
	}
	for _, ai := range added[paired:] {
		d.walk(w.with(index(ai), index(ai)), n.element(-1, ai))
	}
}

func (d *differ) scalar(v any) *Scalar {
	if !d.values || !isScalar(v) {
		return nil
	}
	return &Scalar{Value: v}
}

func (d *differ) emit(w where, a AttrChange) {
	p := w.at
	a.Path = render(p, w.before)
	a.at = p
	a.beforeAt = w.before
	for _, r := range d.replace {
		if p.overlaps(r) || w.before.overlaps(r) {
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

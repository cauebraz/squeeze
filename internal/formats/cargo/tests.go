package cargo

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

type block struct {
	name  string
	lines []string
}

func (p *parser) target(i int) int {
	t := Target{Name: p.pending}
	if t.Name == "" {
		t.Name = fmt.Sprintf("test binary %d", len(p.run.Targets)+1)
	}
	p.pending = ""
	var failed []string
	var blocks []*block
	var cur *block
	counts := map[string]int{}
	list := false
	j := i
	for ; j < len(p.lines); j++ {
		l := p.lines[j]
		if isMessage(l) || rerunRe.MatchString(l) || runningRe.MatchString(l) {
			break
		}
		if m := resultRe.FindStringSubmatch(l); m != nil {
			t.Result = m[1]
			t.Passed, t.Failed, t.Ignored, t.Measured, t.FilteredOut = atoi(m[2]), atoi(m[3]), atoi(m[4]), atoi(m[5]), atoi(m[6])
			j++
			break
		}
		if m := blockRe.FindStringSubmatch(l); m != nil {
			cur, list = &block{name: m[1]}, false
			blocks = append(blocks, cur)
			continue
		}
		if l == "failures:" {
			cur, list = nil, true
			continue
		}
		if cur != nil {
			cur.lines = append(cur.lines, l)
			continue
		}
		if list {
			if name, ok := strings.CutPrefix(l, "    "); ok {
				failed = append(failed, name)
			}
			continue
		}
		if m := testLineRe.FindStringSubmatch(l); m != nil {
			counts[m[2]]++
			if m[2] == "FAILED" {
				failed = append(failed, m[1])
			}
		}
	}
	if t.Result == "" {
		t.Passed, t.Failed, t.Ignored = counts["ok"], counts["FAILED"], counts["ignored"]
	}
	p.run.Targets = append(p.run.Targets, t)
	p.run.Failures = append(p.run.Failures, failures(t.Name, len(p.run.Targets)-1, failed, blocks)...)
	return j
}

func failures(target string, index int, names []string, blocks []*block) []Failure {
	byName := map[string]*Failure{}
	var out []*Failure
	add := func(name string) *Failure {
		if f, ok := byName[name]; ok {
			return f
		}
		f := &Failure{Target: target, Test: core.Clean(name), Message: []string{}, Output: []string{}, index: index}
		byName[name] = f
		out = append(out, f)
		return f
	}
	for _, b := range blocks {
		fill(add(b.name), b.lines)
	}
	for _, n := range names {
		add(strings.TrimSuffix(n, " - should panic"))
	}
	sort.SliceStable(out, func(a, b int) bool { return out[a].Test < out[b].Test })
	res := make([]Failure, 0, len(out))
	for _, f := range out {
		res = append(res, *f)
	}
	return res
}

func fill(f *Failure, lines []string) {
	in := false
	done := false
	for _, l := range lines {
		text := strings.TrimRight(core.Line(l), " ")
		switch {
		case !done && !in && panicRe.MatchString(l):
			m := panicRe.FindStringSubmatch(l)
			f.File, f.Line, f.Column = core.Clean(m[1]), atoi(m[2]), atoi(m[3])
			in = true
		case !done && !in && oldPanicRe.MatchString(l):
			m := oldPanicRe.FindStringSubmatch(l)
			f.File, f.Line, f.Column = core.Clean(m[2]), atoi(m[3]), atoi(m[4])
			f.Message = append(f.Message, core.Clean(m[1]))
			done = true
		case in && (text == "" || strings.HasPrefix(l, "note: run with `RUST_BACKTRACE") || l == "stack backtrace:"):
			in, done = false, true
			if text != "" && !strings.HasPrefix(l, "note: ") {
				f.Output = append(f.Output, text)
			}
		case in:
			f.Message = append(f.Message, text)
		case text == "" || text == "stderr:" || text == "stdout:" || strings.HasPrefix(l, "note: run with `RUST_BACKTRACE"):
		default:
			f.Output = append(f.Output, text)
		}
	}
	if strings.Contains(f.File, "rustdoctest") {
		f.File, f.Line, f.Column = "", 0, 0
	}
}

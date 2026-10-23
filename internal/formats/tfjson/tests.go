package tfjson

import (
	"maps"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

var failingTestStatus = map[string]bool{"fail": true, "error": true}

func (b *builder) tests() *Tests {
	if b.r.Tests == nil {
		b.r.Tests = &Tests{Runs: []TestRun{}, Files: []TestFile{}, Left: []LeftInState{}}
	}
	return b.r.Tests
}

func (b *builder) leftInState(l Line) bool {
	switch {
	case l.Type == "test_cleanup" && l.Cleanup != nil:
		b.left(l, "cleanup_failed", "", l.Cleanup.FailedResources)
		b.left(l, "skip_cleanup", "", l.Cleanup.SkippedResources)
	case l.Type == "test_interrupt" && l.Interrupt != nil:
		b.left(l, "interrupted", "", l.Interrupt.State)
		for _, k := range slices.Sorted(maps.Keys(l.Interrupt.States)) {
			b.left(l, "interrupted", k, l.Interrupt.States[k])
		}
		planned := make([]streamTestResource, 0, len(l.Interrupt.Planned))
		for _, p := range l.Interrupt.Planned {
			planned = append(planned, streamTestResource{Instance: p})
		}
		b.left(l, "planned", "", planned)
	default:
		return false
	}
	return true
}

func (b *builder) left(l Line, reason, state string, resources []streamTestResource) {
	if len(resources) == 0 {
		return
	}
	out := make([]LeftResource, 0, len(resources))
	for _, r := range resources {
		out = append(out, LeftResource{Address: core.Clean(r.Instance), DeposedKey: core.Clean(r.DeposedKey)})
	}
	t := b.tests()
	t.Left = append(t.Left, LeftInState{
		File: core.Clean(l.AtTestFile), Run: core.Clean(l.AtTestRun), Reason: reason, State: core.Clean(state), Resources: out,
	})
}

func (b *builder) testRun(s streamTestRun) {
	s.Path, s.Run, s.Status = core.Clean(s.Path), core.Clean(s.Run), core.Clean(s.Status)
	t := b.tests()
	key := [2]string{s.Path, s.Run}
	i, ok := b.runs[key]
	if !ok {
		i = len(t.Runs)
		b.runs[key] = i
		t.Runs = append(t.Runs, TestRun{File: s.Path, Run: s.Run, Diagnostics: []tfdiag.Diagnostic{}})
	}
	if s.Status != "" {
		t.Runs[i].Status = s.Status
	}
	if s.Elapsed != nil {
		t.Runs[i].ElapsedMs = s.Elapsed
	}
}

func (b *builder) testFile(path, status string) *TestFile {
	t := b.tests()
	i, ok := b.files[path]
	if !ok {
		i = len(t.Files)
		b.files[path] = i
		t.Files = append(t.Files, TestFile{File: path, Diagnostics: []tfdiag.Diagnostic{}})
	}
	if status != "" {
		t.Files[i].Status = status
	}
	return &t.Files[i]
}

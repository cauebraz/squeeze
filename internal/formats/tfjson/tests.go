package tfjson

import (
	"maps"
	"slices"

	"github.com/cauebraz/squeeze/internal/sanitize"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type leftReason string

const (
	leftCleanupFailed leftReason = "cleanup_failed"
	leftSkipCleanup   leftReason = "skip_cleanup"
	leftInterrupted   leftReason = "interrupted"
	leftPlanned       leftReason = "planned"
)

var failingTestStatus = map[string]bool{"fail": true, "error": true}

func (b *builder) tests() *tests {
	if b.r.Tests == nil {
		b.r.Tests = &tests{Runs: []testRun{}, Files: []testFile{}, Left: []leftInState{}}
	}
	return b.r.Tests
}

func (b *builder) leftInState(l inputLine) bool {
	switch {
	case l.Type == "test_cleanup" && l.Cleanup != nil:
		b.left(l, leftCleanupFailed, "", l.Cleanup.FailedResources)
		b.left(l, leftSkipCleanup, "", l.Cleanup.SkippedResources)
	case l.Type == "test_interrupt" && l.Interrupt != nil:
		b.left(l, leftInterrupted, "", l.Interrupt.State)
		for _, k := range slices.Sorted(maps.Keys(l.Interrupt.States)) {
			b.left(l, leftInterrupted, k, l.Interrupt.States[k])
		}
		planned := make([]rawTestResource, 0, len(l.Interrupt.Planned))
		for _, p := range l.Interrupt.Planned {
			planned = append(planned, rawTestResource{Instance: p})
		}
		b.left(l, leftPlanned, "", planned)
	default:
		return false
	}
	return true
}

func (b *builder) left(l inputLine, reason leftReason, state string, resources []rawTestResource) {
	if len(resources) == 0 {
		return
	}
	out := make([]leftResource, 0, len(resources))
	for _, r := range resources {
		out = append(out, leftResource{Address: sanitize.Clean(r.Instance), DeposedKey: sanitize.Clean(r.DeposedKey)})
	}
	t := b.tests()
	t.Left = append(t.Left, leftInState{
		File: sanitize.Clean(l.AtTestFile), Run: sanitize.Clean(l.AtTestRun), Reason: reason, State: sanitize.Clean(state), Resources: out,
	})
}

func (b *builder) testRun(s rawTestRun) {
	s.Path, s.Run, s.Status = sanitize.Clean(s.Path), sanitize.Clean(s.Run), sanitize.Clean(s.Status)
	t := b.tests()
	key := [2]string{s.Path, s.Run}
	i, ok := b.runs[key]
	if !ok {
		i = len(t.Runs)
		b.runs[key] = i
		t.Runs = append(t.Runs, testRun{File: s.Path, Run: s.Run, Diagnostics: []tfdiag.Diagnostic{}})
	}
	if s.Status != "" {
		t.Runs[i].Status = s.Status
	}
	if s.Elapsed != nil {
		t.Runs[i].ElapsedMs = s.Elapsed
	}
}

func (b *builder) testFile(path, status string) *testFile {
	t := b.tests()
	i, ok := b.files[path]
	if !ok {
		i = len(t.Files)
		b.files[path] = i
		t.Files = append(t.Files, testFile{File: path, Diagnostics: []tfdiag.Diagnostic{}})
	}
	if status != "" {
		t.Files[i].Status = status
	}
	return &t.Files[i]
}

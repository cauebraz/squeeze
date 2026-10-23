package formattest

import (
	"bytes"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
)

func CheckStable(t testing.TB, run func() (core.Result, error)) {
	t.Helper()
	first, firstErr := run()
	second, secondErr := run()
	if (firstErr == nil) != (secondErr == nil) || (firstErr != nil && firstErr.Error() != secondErr.Error()) {
		t.Fatalf("error differs between runs: %v, then %v", firstErr, secondErr)
	}
	if firstErr != nil {
		return
	}
	if first.Text != second.Text {
		t.Fatalf("text differs between runs:\n%s\nthen:\n%s", first.Text, second.Text)
	}
	a, errA := core.Envelope("stable", 0, first, nil)
	b, errB := core.Envelope("stable", 0, second, nil)
	if (errA == nil) != (errB == nil) {
		t.Fatalf("envelope error differs between runs: %v, then %v", errA, errB)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("envelope differs between runs:\n%s\nthen:\n%s", a, b)
	}
}

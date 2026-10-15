package buildkit

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

var words = map[string]string{"failed": "fail", "canceled": "canceled", "cached": "cached", "done": "done", "running": "running"}

func Text(r Report) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, w := range r.Warnings {
		b.WriteString("warning: " + w.Message + "\n")
	}
	for _, s := range r.Steps {
		fmt.Fprintf(&b, "%s %s\n", words[s.Status], s.Name)
		for _, l := range s.Output {
			b.WriteString("  " + l + "\n")
		}
		if s.Error != "" && !repeated(s.Error, r.Errors) {
			b.WriteString("  error: " + s.Error + "\n")
		}
	}
	for i, e := range r.Errors {
		if i == 0 && r.Location != "" {
			e = r.Location + ": " + e
		}
		b.WriteString("error: " + e + "\n")
	}
	if r.OtherSteps > 0 {
		fmt.Fprintf(&b, "... %d other %s (--all)\n", r.OtherSteps, core.Plural(r.OtherSteps, "step", "steps"))
	}
	return b.String()
}

func repeated(err string, final []string) bool {
	for _, e := range final {
		if strings.HasSuffix(e, err) {
			return true
		}
	}
	return false
}

func header(r Report) string {
	line := fmt.Sprintf("buildkit: %d %s", r.TotalSteps, core.Plural(r.TotalSteps, "step", "steps"))
	for _, c := range []struct {
		n    int
		word string
	}{{r.CachedSteps, "cached"}, {r.FailedSteps, "failed"}, {r.CanceledSteps, "canceled"}} {
		if c.n > 0 {
			line += fmt.Sprintf(", %d %s", c.n, c.word)
		}
	}
	if n := len(r.Warnings); n > 0 {
		line += fmt.Sprintf("; %d %s", n, core.Plural(n, "warning", "warnings"))
	}
	return line
}

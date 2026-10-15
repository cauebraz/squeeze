package precommit

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

func label(h Hook) string {
	if h.ID == "" || h.ID == h.Name {
		return h.Name
	}
	return h.ID + " (" + h.Name + ")"
}

func verdict(h Hook) string {
	switch h.Status {
	case Passed:
		return "pass " + label(h)
	case Skipped:
		if h.NoFiles {
			return "skip " + label(h) + ": no files to check"
		}
		return "skip " + label(h) + ": SKIP"
	case Unfinished:
		return "unfinished " + label(h)
	case Failed:
	}
	var why []string
	if h.ExitCode != 0 {
		why = append(why, fmt.Sprintf("exit code %d", h.ExitCode))
	}
	if h.Modified {
		why = append(why, "files modified")
	}
	line := "fail " + label(h)
	if len(why) > 0 {
		line += ": " + strings.Join(why, ", ")
	}
	return line
}

func indent(b *strings.Builder, lines []string) {
	for _, l := range lines {
		if l == "" {
			b.WriteByte('\n')
			continue
		}
		b.WriteString("  " + l + "\n")
	}
}

func Text(r Report, o Options) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, e := range r.Errors {
		b.WriteString("error: " + e + "\n")
	}
	for _, w := range r.Warnings {
		b.WriteString("warning: " + w + "\n")
	}
	for _, h := range r.Hooks {
		b.WriteString(verdict(h.Hook) + "\n")
		indent(&b, h.Output)
	}
	if len(r.Other) > 0 {
		b.WriteString("other output:\n")
		indent(&b, r.Other)
	}
	for _, f := range r.ChangedFiles {
		b.WriteString("changed: " + f + "\n")
	}
	if o.All && len(r.Diff) > 0 {
		b.WriteString("diff:\n")
		indent(&b, r.Diff)
	}
	if r.DiffOmitted > 0 {
		fmt.Fprintf(&b, "... %d diff %s (--all)\n", r.DiffOmitted, core.Plural(r.DiffOmitted, "line", "lines"))
	}
	if r.HooksOmitted > 0 {
		fmt.Fprintf(&b, "... %d passed or skipped %s (--all)\n", r.HooksOmitted, core.Plural(r.HooksOmitted, "hook", "hooks"))
	}
	return b.String()
}

func header(r Report) string {
	line := fmt.Sprintf("%s: %d %s", core.ToolLabel("pre-commit", r.ToolVersion), r.Total, core.Plural(r.Total, "hook", "hooks"))
	var parts []string
	for _, c := range []struct {
		n    int
		what string
	}{{r.Failed, "failed"}, {r.Unfinished, "unfinished"}, {r.Passed, "passed"}, {r.Skipped, "skipped"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, c.what))
		}
	}
	if len(parts) > 0 {
		line += ", " + strings.Join(parts, ", ")
	}
	if r.Modified > 0 {
		line += fmt.Sprintf("; %d %s modified files", r.Modified, core.Plural(r.Modified, "hook", "hooks"))
	}
	return line
}

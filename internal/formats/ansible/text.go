package ansible

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

func Text(r Report, o Options) string {
	var b strings.Builder
	b.WriteString(header(r) + "\n")
	for _, h := range r.Hosts {
		fmt.Fprintf(&b, "  %s: ok=%d changed=%d unreachable=%d failed=%d skipped=%d rescued=%d ignored=%d\n",
			h.Host, h.OK, h.Changed, h.Unreachable, h.Failed, h.Skipped, h.Rescued, h.Ignored)
	}
	if r.HostsOmitted > 0 {
		fmt.Fprintf(&b, "  ... %d %s (--max-hosts 0)\n", r.HostsOmitted, core.Plural(r.HostsOmitted, "host", "hosts"))
	}
	for _, e := range r.Errors {
		b.WriteString("error: " + e + "\n")
	}
	for _, w := range r.Warnings {
		b.WriteString("warning: " + w + "\n")
	}
	for _, t := range r.Tasks {
		b.WriteString(taskLine(t) + "\n")
		writeDetail(&b, t.Detail, o.MaxLines)
	}
	if r.TasksOmitted > 0 {
		fmt.Fprintf(&b, "... %d %s (--max-tasks 0)\n", r.TasksOmitted, core.Plural(r.TasksOmitted, "task", "tasks"))
	}
	if !o.All {
		var parts []string
		for _, c := range []struct {
			n         int
			one, many string
		}{
			{r.Changed, "changed task", "changed tasks"},
			{r.Ignored, "ignored failure", "ignored failures"},
			{r.Rescued, "rescued failure", "rescued failures"},
			{r.Recovered, "ignored or rescued failure", "ignored or rescued failures"},
		} {
			if c.n > 0 {
				parts = append(parts, fmt.Sprintf("%d %s", c.n, core.Plural(c.n, c.one, c.many)))
			}
		}
		if len(parts) > 0 {
			fmt.Fprintf(&b, "... %s (--all)\n", strings.Join(parts, ", "))
		}
	}
	return b.String()
}

func header(r Report) string {
	line := fmt.Sprintf("%s: %d %s", core.ToolLabel("ansible-playbook", r.ToolVersion), r.Plays, core.Plural(r.Plays, "play", "plays"))
	if r.Recap {
		n := len(r.Hosts) + r.HostsOmitted
		line += fmt.Sprintf(", %d %s", n, core.Plural(n, "host", "hosts"))
	}
	if r.Failed > 0 {
		line += fmt.Sprintf(", %d failed", r.Failed)
	}
	if r.Unreachable > 0 {
		line += fmt.Sprintf(", %d unreachable", r.Unreachable)
	}
	if !r.Recap {
		line += ", no PLAY RECAP"
	}
	var parts []string
	if n := len(r.Errors); n > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", n, core.Plural(n, "error", "errors")))
	}
	if n := len(r.Warnings); n > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", n, core.Plural(n, "warning", "warnings")))
	}
	if len(parts) > 0 {
		line += "; " + strings.Join(parts, ", ")
	}
	return line
}

func taskLine(t Task) string {
	label := t.Task
	if label == "" {
		label = "task"
	}
	line := fmt.Sprintf("%s %s: %s", t.Status, t.Host, label)
	if t.Item != "" {
		line += " (item=" + t.Item + ")"
	}
	if t.Path != "" {
		line += " " + t.Path
	}
	return line
}

func writeDetail(b *strings.Builder, d Detail, max int) {
	for _, s := range []struct{ label, value string }{
		{"msg", d.Msg}, {"rc", d.RC}, {"stderr", d.Stderr}, {"stdout", d.Stdout},
		{"module_stderr", d.ModuleStderr}, {"module_stdout", d.ModuleStdout}, {"censored", d.Censored},
	} {
		value := strings.TrimRight(s.value, "\r\n")
		if strings.TrimSpace(value) == "" {
			continue
		}
		lines := strings.Split(value, "\n")
		if len(lines) == 1 {
			b.WriteString("  " + s.label + ": " + core.Clean(value) + "\n")
			continue
		}
		b.WriteString("  " + s.label + ":\n")
		kept, cut := core.LimitTail(lines, max)
		if cut > 0 {
			fmt.Fprintf(b, "    ... %d %s cut (--max-lines 0)\n", cut, core.Plural(cut, "line", "lines"))
		}
		for _, l := range kept {
			b.WriteString("    " + core.Line(l) + "\n")
		}
	}
}

package junit

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

func Text(r Report) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, c := range r.Cases {
		kind := "fail"
		if c.Kind == "error" {
			kind = "error"
		}
		name := label(c)
		if strings.Contains(name, c.Message) {
			fmt.Fprintf(&b, "%s %s\n", kind, name)
		} else {
			fmt.Fprintf(&b, "%s %s: %s\n", kind, name, core.Ellipsis(c.Message, maxMessage))
		}
		writeBlock(&b, "  ", c.Text)
		writeStream(&b, "  ", "stdout", c.Stdout)
		writeStream(&b, "  ", "stderr", c.Stderr)
	}
	if r.CasesOmitted > 0 {
		fmt.Fprintf(&b, "... %d more failing %s (--max-cases 0)\n", r.CasesOmitted, core.Plural(r.CasesOmitted, "case", "cases"))
	}
	for _, s := range r.SuiteOutput {
		name := s.Suite
		if name == "" {
			name = "without a name"
		}
		writeStream(&b, "", "suite "+name+" stdout", s.Stdout)
		writeStream(&b, "", "suite "+name+" stderr", s.Stderr)
	}
	return b.String()
}

func label(c FailedCase) string {
	name := c.Name
	if name == "" {
		name = "(no name)"
	}
	if c.Classname == "" || strings.HasPrefix(c.Name, c.Classname) {
		return name
	}
	return c.Classname + " " + name
}

func writeBlock(b *strings.Builder, indent string, blk Block) {
	for _, l := range blk.Lines {
		b.WriteString(indent + l + "\n")
	}
	if blk.LinesOmitted > 0 {
		fmt.Fprintf(b, "%s... %d more %s (--max-lines 0)\n", indent, blk.LinesOmitted, core.Plural(blk.LinesOmitted, "line", "lines"))
	}
}

func writeStream(b *strings.Builder, indent, name string, blk Block) {
	if len(blk.Lines) == 0 {
		return
	}
	b.WriteString(indent + name + ":\n")
	writeBlock(b, indent+"  ", blk)
}

func header(r Report) string {
	line := fmt.Sprintf("junit: %d %s in %d %s", r.Tests, core.Plural(r.Tests, "test", "tests"), r.Suites, core.Plural(r.Suites, "suite", "suites"))
	var parts []string
	for _, c := range []struct {
		n         int
		one, many string
	}{{r.Passed, "passed", "passed"}, {r.Failures, "failure", "failures"}, {r.Errors, "error", "errors"}, {r.Skipped, "skipped", "skipped"}, {r.Flaky, "flaky", "flaky"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, core.Plural(c.n, c.one, c.many)))
		}
	}
	if len(parts) > 0 {
		line += ": " + strings.Join(parts, ", ")
	}
	if len(r.Mismatches) > 0 {
		var said []string
		for _, m := range r.Mismatches {
			said = append(said, fmt.Sprintf("%d %s", m.Attribute, m.Count))
		}
		line += "; the report's attributes say " + strings.Join(said, ", ")
	}
	return line
}

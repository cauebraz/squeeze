package junit

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

func render(r report) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, c := range r.Cases {
		kind := "fail"
		switch c.Kind {
		case kindError:
			kind = "error"
		case kindFailure:
		}
		name := label(c)
		if messageInName(c, name) {
			fmt.Fprintf(&b, "%s %s\n", kind, name)
		} else {
			fmt.Fprintf(&b, "%s %s: %s\n", kind, name, core.Ellipsis(c.Message, maxMessage))
		}
		c.Text.Write(&b, "  ", "--max-output")
		writeStream(&b, "  ", "stdout", c.Stdout)
		writeStream(&b, "  ", "stderr", c.Stderr)
	}
	if r.TestsOmitted > 0 {
		fmt.Fprintf(&b, "... %d more failed %s (--max-tests 0)\n", r.TestsOmitted, core.Plural(r.TestsOmitted, "test", "tests"))
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

func label(c failedCase) string {
	name := c.Name
	if name == "" {
		name = "(no name)"
	}
	if c.Classname == "" || strings.HasPrefix(c.Name, c.Classname) {
		return name
	}
	return c.Classname + " " + name
}

func messageInName(c failedCase, label string) bool {
	return c.Message == "" || c.Message == c.Name || c.Message == label || strings.HasSuffix(c.Name, " "+c.Message)
}

func writeStream(b *strings.Builder, indent, name string, blk core.Block) {
	if len(blk.Lines) == 0 {
		return
	}
	b.WriteString(indent + name + ":\n")
	blk.Write(b, indent+"  ", "--max-output")
}

func header(r report) string {
	line := fmt.Sprintf("junit: %d %s in %d %s", r.Tests, core.Plural(r.Tests, "test", "tests"), r.Suites, core.Plural(r.Suites, "suite", "suites"))
	if parts := core.JoinCounts(
		core.Part{N: r.Passed, One: "passed"},
		core.Part{N: r.Failures, One: "failure", Many: "failures"},
		core.Part{N: r.Errors, One: "error", Many: "errors"},
		core.Part{N: r.Skipped, One: "skipped"},
		core.Part{N: r.Flaky, One: "flaky"},
	); parts != "" {
		line += ": " + parts
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

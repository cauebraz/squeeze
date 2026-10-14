package tfstate

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

const maxValueBytes = 200

func Text(r Report) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, o := range r.Outputs {
		value := "(sensitive)"
		if !o.Sensitive {
			value = core.Ellipsis(core.Line(string(o.Value)), maxValueBytes)
		}
		fmt.Fprintf(&b, "%s = %s\n", core.Line(o.Name), value)
	}
	if r.Omitted > 0 {
		fmt.Fprintf(&b, "... %d more %s (--max-outputs 0)\n", r.Omitted, core.Plural(r.Omitted, "output", "outputs"))
	}
	return b.String()
}

func header(r Report) string {
	line := fmt.Sprintf("%s output: %d %s", r.Tool, r.Total, core.Plural(r.Total, "output", "outputs"))
	if r.Sensitive > 0 {
		line += fmt.Sprintf(", %d sensitive", r.Sensitive)
	}
	return line
}

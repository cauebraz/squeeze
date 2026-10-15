package tfstate

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

const maxValueBytes = 200

func Text(r Report) string {
	if r.Source == "resource" {
		return showText(r)
	}
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	if s := r.State; s != nil {
		for _, res := range s.Resources {
			b.WriteString(res.Address)
			if res.DeposedKey != "" {
				b.WriteString(" (deposed " + res.DeposedKey + ")")
			}
			if res.Tainted {
				b.WriteString(" (tainted)")
			}
			b.WriteByte('\n')
		}
		if s.ResourcesOmitted > 0 {
			fmt.Fprintf(&b, "... %d more %s (--max-resources 0)\n", s.ResourcesOmitted, core.Plural(s.ResourcesOmitted, "resource", "resources"))
		}
		if len(r.Outputs) > 0 {
			b.WriteString("outputs:\n")
		}
	}
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
	outputs := fmt.Sprintf("%d %s", r.Total, core.Plural(r.Total, "output", "outputs"))
	if r.Sensitive > 0 {
		outputs += fmt.Sprintf(", %d sensitive", r.Sensitive)
	}
	s := r.State
	if s == nil {
		return r.Tool + " output: " + outputs
	}
	parts := []string{fmt.Sprintf("%d %s", s.Managed, core.Plural(s.Managed, "resource", "resources"))}
	if s.DataSources > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", s.DataSources, core.Plural(s.DataSources, "data source", "data sources")))
	}
	if s.Tainted > 0 {
		parts = append(parts, fmt.Sprintf("%d tainted", s.Tainted))
	}
	if s.Deposed > 0 {
		parts = append(parts, fmt.Sprintf("%d deposed", s.Deposed))
	}
	line := core.ToolLabel(r.Tool, r.ToolVersion) + " state: " + strings.Join(parts, ", ")
	if r.Total > 0 {
		line += "; " + outputs
	}
	return line
}

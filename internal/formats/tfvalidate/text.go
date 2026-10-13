package tfvalidate

import (
	"fmt"
	"github.com/cauebraz/squeeze/internal/core"
	"strings"

	"github.com/cauebraz/squeeze/internal/tfdiag"
)

func Text(r Report, o Options) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, d := range r.Diagnostics {
		tfdiag.Write(&b, d, tfdiag.WriteOptions{NoDetail: o.NoDetail})
	}
	if r.Omitted > 0 {
		fmt.Fprintf(&b, "... %d more %s (--max-diagnostics 0)\n", r.Omitted, core.Plural(r.Omitted, "diagnostic", "diagnostics"))
	}
	return b.String()
}

func header(r Report) string {
	state := "valid"
	if !r.Valid {
		state = "invalid"
	}
	var parts []string
	if r.Errors > 0 || !r.Valid {
		parts = append(parts, fmt.Sprintf("%d %s", r.Errors, core.Plural(r.Errors, "error", "errors")))
	}
	if r.Warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", r.Warnings, core.Plural(r.Warnings, "warning", "warnings")))
	}
	line := r.Tool + " validate: " + state
	if len(parts) > 0 {
		line += ", " + strings.Join(parts, ", ")
	}
	return line
}

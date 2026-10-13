package tfvalidate

import (
	"fmt"
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
		fmt.Fprintf(&b, "... %d more %s (--max-diagnostics)\n", r.Omitted, plural(r.Omitted, "diagnostic", "diagnostics"))
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
		parts = append(parts, fmt.Sprintf("%d %s", r.Errors, plural(r.Errors, "error", "errors")))
	}
	if r.Warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", r.Warnings, plural(r.Warnings, "warning", "warnings")))
	}
	line := r.Tool + " validate: " + state
	if len(parts) > 0 {
		line += ", " + strings.Join(parts, ", ")
	}
	return line
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

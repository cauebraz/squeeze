package tfvalidate

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"

	"github.com/cauebraz/squeeze/internal/tfdiag"
)

func text(r report, o options) string {
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

func header(r report) string {
	state := "valid"
	if !r.Valid {
		state = "invalid"
	}
	parts := tfdiag.CountWords(r.Errors, r.Warnings)
	if !r.Valid && r.Errors == 0 {
		parts = append([]string{"0 errors"}, parts...)
	}
	line := r.Tool + " validate: " + state
	if len(parts) > 0 {
		line += ", " + strings.Join(parts, ", ")
	}
	return line
}

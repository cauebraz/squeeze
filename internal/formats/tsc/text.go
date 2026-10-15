package tsc

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

func Text(r Report) string {
	var b strings.Builder
	b.WriteString(header(r))
	b.WriteByte('\n')
	for _, d := range r.Diagnostics {
		tfdiag.Write(&b, d, tfdiag.WriteOptions{})
	}
	if r.ErrorsOmitted > 0 {
		fmt.Fprintf(&b, "... %d more %s, %s (--max-errors 0)\n", r.ErrorsOmitted, core.Plural(r.ErrorsOmitted, "error", "errors"), core.TopCounts(r.OmittedByCode, maxCodesInNotice, "code", "codes"))
	}
	switch {
	case r.all:
		for _, l := range r.Other {
			b.WriteString("other: " + l + "\n")
		}
	case len(r.Other) > 0:
		fmt.Fprintf(&b, "... %d other %s (--all)\n", len(r.Other), core.Plural(len(r.Other), "line", "lines"))
	}
	return b.String()
}

func header(r Report) string {
	line := fmt.Sprintf("tsc: %d %s", r.Errors, core.Plural(r.Errors, "error", "errors"))
	if r.Warnings > 0 {
		line += fmt.Sprintf(", %d %s", r.Warnings, core.Plural(r.Warnings, "warning", "warnings"))
	}
	if r.Messages > 0 {
		line += fmt.Sprintf(", %d %s", r.Messages, core.Plural(r.Messages, "message", "messages"))
	}
	if r.Files > 0 {
		line += fmt.Sprintf(" in %d %s", r.Files, core.Plural(r.Files, "file", "files"))
	}
	if r.SummaryMismatch {
		line += "; tsc's summary differs: " + r.Summary
	}
	return line
}

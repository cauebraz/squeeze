package tsc

import (
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const maxCodesInNotice = 5

type Report struct {
	Tool            string              `json:"tool"`
	ToolVersion     string              `json:"tool_version"`
	Errors          int                 `json:"errors"`
	Warnings        int                 `json:"warnings"`
	Messages        int                 `json:"messages"`
	Files           int                 `json:"files"`
	Summary         string              `json:"summary"`
	SummaryErrors   int                 `json:"summary_errors"`
	SummaryFiles    int                 `json:"summary_files"`
	SummaryMismatch bool                `json:"summary_mismatch"`
	Codes           []core.Count        `json:"codes"`
	Diagnostics     []tfdiag.Diagnostic `json:"diagnostics"`
	ErrorsOmitted   int                 `json:"errors_omitted"`
	OmittedByCode   []core.Count        `json:"omitted_by_code"`
	Other           []string            `json:"other"`
	all             bool
}

func Build(l *Log, o Options) Report {
	r := Report{Tool: "tsc", Summary: l.Summary, SummaryErrors: l.SummaryErrors, SummaryFiles: l.SummaryFiles, Other: append([]string{}, l.Other...), all: o.All}
	files, errorFiles := map[string]bool{}, map[string]bool{}
	codes := map[string]int{}
	coded := 0
	for _, d := range l.Diagnostics {
		switch d.Severity {
		case "error":
			r.Errors++
		case "warning":
			r.Warnings++
		default:
			r.Messages++
		}
		if d.File != "" {
			files[d.File] = true
		}
		if strings.HasPrefix(d.Address, "TS") {
			codes[d.Address]++
			if d.Severity == "error" {
				coded++
				if d.File != "" {
					errorFiles[d.File] = true
				}
			}
		}
	}
	r.Files = len(files)
	r.Codes = core.SortedCounts(codes)
	if r.Summary != "" {
		r.SummaryMismatch = r.SummaryErrors != coded || (r.SummaryFiles > 0 && r.SummaryFiles != len(errorFiles))
	}
	all := tfdiag.Normalize(l.Diagnostics)
	kept, _ := core.Limit(all, o.MaxErrors)
	r.Diagnostics = append([]tfdiag.Diagnostic{}, kept...)
	cut := map[string]int{}
	for _, d := range all[len(kept):] {
		name := d.Address
		if name == "" {
			name = "no code"
		}
		cut[name] += max(d.Count, 1)
		r.ErrorsOmitted += max(d.Count, 1)
	}
	r.OmittedByCode = core.SortedCounts(cut)
	return r
}

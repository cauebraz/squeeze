package tfvalidate

import (
	"errors"
	"fmt"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/logscan"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

var errMissingValid = errors.New("missing valid")

type rawValidation struct {
	FormatVersion string                `json:"format_version"`
	Valid         *bool                 `json:"valid"`
	ErrorCount    int                   `json:"error_count"`
	WarningCount  int                   `json:"warning_count"`
	Diagnostics   []tfdiag.Raw          `json:"diagnostics"`
	Trailing      []logscan.FlaggedLine `json:"-"`
}

type report struct {
	Tool        string              `json:"tool"`
	ToolVersion string              `json:"tool_version"`
	Valid       bool                `json:"valid"`
	Errors      int                 `json:"errors"`
	Warnings    int                 `json:"warnings"`
	Diagnostics []tfdiag.Diagnostic `json:"diagnostics"`
	Omitted     int                 `json:"diagnostics_omitted"`
}

func parse(input []byte) (*rawValidation, error) {
	var v rawValidation
	rest, err := core.DecodeJSON(input, &v)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if err := tfdiag.CheckFormatVersion(v.FormatVersion); err != nil {
		return nil, err
	}
	if v.Valid == nil {
		return nil, errMissingValid
	}
	v.Trailing = logscan.FlaggedLines(rest)
	return &v, nil
}

func build(v *rawValidation, o options) report {
	ds := make([]tfdiag.Diagnostic, 0, len(v.Diagnostics))
	for _, raw := range v.Diagnostics {
		ds = append(ds, tfdiag.Convert(raw))
	}
	errorCount, warningCount := tfdiag.Count(ds)
	errorCount, warningCount = max(v.ErrorCount, errorCount), max(v.WarningCount, warningCount)
	trailing := tfdiag.FromLines(v.Trailing, "")
	trailingErrors, trailingWarnings := tfdiag.Count(trailing)
	errorCount += trailingErrors
	warningCount += trailingWarnings
	valid := *v.Valid && trailingErrors == 0
	ds = tfdiag.Normalize(append(ds, trailing...))
	kept, omitted := tfdiag.Limit(ds, o.MaxDiagnostics)
	return report{
		Tool:        "terraform",
		Valid:       valid,
		Errors:      errorCount,
		Warnings:    warningCount,
		Diagnostics: kept,
		Omitted:     omitted,
	}
}

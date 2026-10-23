package tfvalidate

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

var (
	ErrMissingVersion = errors.New("missing format_version")
	ErrMissingValid   = errors.New("missing valid")
)

type Validation struct {
	FormatVersion string             `json:"format_version"`
	Valid         *bool              `json:"valid"`
	ErrorCount    int                `json:"error_count"`
	WarningCount  int                `json:"warning_count"`
	Diagnostics   []tfdiag.Raw       `json:"diagnostics"`
	Trailing      []core.FlaggedLine `json:"-"`
}

type Report struct {
	Tool        string              `json:"tool"`
	ToolVersion string              `json:"tool_version"`
	Valid       bool                `json:"valid"`
	Errors      int                 `json:"errors"`
	Warnings    int                 `json:"warnings"`
	Diagnostics []tfdiag.Diagnostic `json:"diagnostics"`
	Omitted     int                 `json:"diagnostics_omitted"`
}

func Parse(input []byte) (*Validation, error) {
	var v Validation
	rest, err := core.DecodeJSON(input, &v)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if v.FormatVersion == "" {
		return nil, ErrMissingVersion
	}
	if major, _, _ := strings.Cut(v.FormatVersion, "."); major != "1" {
		return nil, fmt.Errorf("unsupported format_version %q", v.FormatVersion)
	}
	if v.Valid == nil {
		return nil, ErrMissingValid
	}
	v.Trailing = core.FlaggedLines(rest)
	return &v, nil
}

func Build(v *Validation, o Options) Report {
	ds := make([]tfdiag.Diagnostic, 0, len(v.Diagnostics))
	for _, raw := range v.Diagnostics {
		ds = append(ds, tfdiag.Convert(raw))
	}
	errorCount, warningCount := tfdiag.Count(ds)
	errorCount, warningCount = max(v.ErrorCount, errorCount), max(v.WarningCount, warningCount)
	valid := *v.Valid
	for _, l := range v.Trailing {
		d := tfdiag.Diagnostic{Severity: "warning", Summary: l.Text}
		if l.Error {
			d.Severity = "error"
			errorCount++
			valid = false
		} else {
			warningCount++
		}
		ds = append(ds, d)
	}
	ds = tfdiag.Normalize(ds)
	kept, omitted := tfdiag.Limit(ds, o.MaxDiagnostics)
	return Report{
		Tool:        "terraform",
		Valid:       valid,
		Errors:      errorCount,
		Warnings:    warningCount,
		Diagnostics: kept,
		Omitted:     omitted,
	}
}

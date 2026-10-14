package tfvalidate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/tfdiag"
)

var (
	ErrMissingVersion = errors.New("missing format_version")
	ErrMissingValid   = errors.New("missing valid")
)

type Validation struct {
	FormatVersion string       `json:"format_version"`
	Valid         *bool        `json:"valid"`
	ErrorCount    int          `json:"error_count"`
	WarningCount  int          `json:"warning_count"`
	Diagnostics   []tfdiag.Raw `json:"diagnostics"`
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
	if err := json.NewDecoder(bytes.NewReader(input)).Decode(&v); err != nil {
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
	return &v, nil
}

func Build(v *Validation, o Options) Report {
	ds := make([]tfdiag.Diagnostic, 0, len(v.Diagnostics))
	for _, raw := range v.Diagnostics {
		ds = append(ds, tfdiag.Convert(raw))
	}
	ds = tfdiag.Normalize(ds)
	errorCount, warningCount := tfdiag.Count(ds)
	kept, omitted := tfdiag.Limit(ds, o.MaxDiagnostics)
	return Report{
		Tool:        "terraform",
		Valid:       *v.Valid,
		Errors:      max(v.ErrorCount, errorCount),
		Warnings:    max(v.WarningCount, warningCount),
		Diagnostics: kept,
		Omitted:     omitted,
	}
}

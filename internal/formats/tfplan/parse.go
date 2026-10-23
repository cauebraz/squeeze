package tfplan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

var (
	ErrMissingVersion = errors.New("missing format_version")
	ErrStateNotPlan   = errors.New("input is terraform state, not a plan")
)

func Parse(input []byte) (*Plan, error) {
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.UseNumber()
	var p Plan
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if p.FormatVersion == "" {
		return nil, ErrMissingVersion
	}
	if major, _, _ := strings.Cut(p.FormatVersion, "."); major != "1" {
		return nil, fmt.Errorf("unsupported format_version %q", p.FormatVersion)
	}
	if p.PlannedValues == nil && p.ResourceChanges == nil && p.Values != nil {
		return nil, ErrStateNotPlan
	}
	p.Trailing = core.FlaggedLines(input[dec.InputOffset():])
	return &p, nil
}

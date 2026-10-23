package tfplan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cauebraz/squeeze/internal/logscan"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

var errStateNotPlan = errors.New("input is terraform state, not a plan")

func parse(input []byte) (*rawPlan, error) {
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.UseNumber()
	var p rawPlan
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if err := tfdiag.CheckFormatVersion(p.FormatVersion); err != nil {
		return nil, err
	}
	if p.PlannedValues == nil && p.ResourceChanges == nil && p.Values != nil {
		return nil, errStateNotPlan
	}
	p.Trailing = logscan.FlaggedLines(input[dec.InputOffset():])
	return &p, nil
}

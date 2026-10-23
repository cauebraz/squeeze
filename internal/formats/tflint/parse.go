package tflint

import (
	"errors"
	"fmt"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/logscan"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

var errNotTFLint = errors.New("input has no tflint issues or errors arrays")

type rawRule struct {
	Name     string `json:"name"`
	Severity string `json:"severity"`
	Link     string `json:"link"`
}

type rawIssue struct {
	Rule    rawRule        `json:"rule"`
	Message string         `json:"message"`
	Range   tfdiag.Range   `json:"range"`
	Callers []tfdiag.Range `json:"callers"`
	Fixable bool           `json:"fixable"`
	Fixed   bool           `json:"fixed"`
}

type rawToolError struct {
	Summary  string        `json:"summary"`
	Message  string        `json:"message"`
	Severity string        `json:"severity"`
	Range    *tfdiag.Range `json:"range"`
}

type rawOutput struct {
	Issues   *[]rawIssue           `json:"issues"`
	Errors   *[]rawToolError       `json:"errors"`
	Trailing []logscan.FlaggedLine `json:"-"`
}

func parse(input []byte) (*rawOutput, error) {
	var out rawOutput
	rest, err := core.DecodeJSON(input, &out)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if out.Issues == nil || out.Errors == nil {
		return nil, errNotTFLint
	}
	out.Trailing = logscan.FlaggedLines(rest)
	return &out, nil
}

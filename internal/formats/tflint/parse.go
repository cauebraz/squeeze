package tflint

import (
	"errors"
	"fmt"

	"github.com/cauebraz/squeeze/internal/core"
)

var ErrNotTFLint = errors.New("input has no tflint issues or errors arrays")

type Pos struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type Range struct {
	Filename string `json:"filename"`
	Start    Pos    `json:"start"`
	End      Pos    `json:"end"`
}

type Rule struct {
	Name     string `json:"name"`
	Severity string `json:"severity"`
	Link     string `json:"link"`
}

type RawIssue struct {
	Rule    Rule    `json:"rule"`
	Message string  `json:"message"`
	Range   Range   `json:"range"`
	Callers []Range `json:"callers"`
	Fixable bool    `json:"fixable"`
	Fixed   bool    `json:"fixed"`
}

type ToolError struct {
	Summary  string `json:"summary"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
	Range    *Range `json:"range"`
}

type Output struct {
	Issues   *[]RawIssue        `json:"issues"`
	Errors   *[]ToolError       `json:"errors"`
	Trailing []core.FlaggedLine `json:"-"`
}

func Parse(input []byte) (*Output, error) {
	var out Output
	rest, err := core.DecodeJSON(input, &out)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if out.Issues == nil || out.Errors == nil {
		return nil, ErrNotTFLint
	}
	out.Trailing = core.FlaggedLines(rest)
	return &out, nil
}

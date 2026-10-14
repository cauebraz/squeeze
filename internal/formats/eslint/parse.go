package eslint

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrNotESLint = errors.New("input is not an ESLint results array")

type Message struct {
	RuleID   *string          `json:"ruleId"`
	Severity int              `json:"severity"`
	Fatal    bool             `json:"fatal"`
	Message  string           `json:"message"`
	Line     int              `json:"line"`
	Column   int              `json:"column"`
	Fix      *json.RawMessage `json:"fix"`
}

type Result struct {
	FilePath            string     `json:"filePath"`
	Messages            *[]Message `json:"messages"`
	SuppressedMessages  []Message  `json:"suppressedMessages"`
	ErrorCount          int        `json:"errorCount"`
	FatalErrorCount     int        `json:"fatalErrorCount"`
	WarningCount        int        `json:"warningCount"`
	FixableErrorCount   int        `json:"fixableErrorCount"`
	FixableWarningCount int        `json:"fixableWarningCount"`
}

type Output struct {
	Results []Result
	Cwd     string
}

func Parse(input []byte) (*Output, error) {
	trimmed := bytes.TrimSpace(input)
	var out Output
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var doc struct {
			Results  *[]Result `json:"results"`
			Metadata struct {
				Cwd string `json:"cwd"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal(trimmed, &doc); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		if doc.Results == nil {
			return nil, ErrNotESLint
		}
		out.Results, out.Cwd = *doc.Results, doc.Metadata.Cwd
	} else if err := json.Unmarshal(trimmed, &out.Results); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	for _, r := range out.Results {
		if r.FilePath == "" || r.Messages == nil {
			return nil, ErrNotESLint
		}
	}
	return &out, nil
}

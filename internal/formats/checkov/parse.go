package checkov

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrNotCheckov = errors.New("input has no checkov report or summary")

type CheckResult struct {
	Result          string `json:"result"`
	SuppressComment string `json:"suppress_comment"`
}

type Record struct {
	CheckID             string      `json:"check_id"`
	CheckName           string      `json:"check_name"`
	CheckResult         CheckResult `json:"check_result"`
	FilePath            string      `json:"file_path"`
	FileLineRange       []int       `json:"file_line_range"`
	Resource            string      `json:"resource"`
	Severity            *string     `json:"severity"`
	Guideline           *string     `json:"guideline"`
	CallerFilePath      *string     `json:"caller_file_path"`
	CallerFileLineRange []int       `json:"caller_file_line_range"`
}

type Summary struct {
	Passed         int    `json:"passed"`
	Failed         int    `json:"failed"`
	Skipped        int    `json:"skipped"`
	ParsingErrors  int    `json:"parsing_errors"`
	ResourceCount  int    `json:"resource_count"`
	CheckovVersion string `json:"checkov_version"`
}

type Results struct {
	PassedChecks  []Record `json:"passed_checks"`
	FailedChecks  []Record `json:"failed_checks"`
	SkippedChecks []Record `json:"skipped_checks"`
	ParsingErrors []string `json:"parsing_errors"`
}

type RawReport struct {
	CheckType string   `json:"check_type"`
	Results   *Results `json:"results"`
	Summary   *Summary `json:"summary"`
}

func Parse(input []byte) ([]RawReport, error) {
	trimmed := bytes.TrimLeft(input, " \t\r\n")
	if len(trimmed) == 0 {
		return nil, ErrNotCheckov
	}
	if trimmed[0] == '[' {
		var reports []RawReport
		if err := json.Unmarshal(trimmed, &reports); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		for _, r := range reports {
			if r.Results == nil || r.Summary == nil {
				return nil, ErrNotCheckov
			}
		}
		return reports, nil
	}
	var probe struct {
		RawReport
		Summary
	}
	if err := json.NewDecoder(bytes.NewReader(trimmed)).Decode(&probe); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if probe.Results != nil && probe.RawReport.Summary != nil {
		return []RawReport{probe.RawReport}, nil
	}
	if probe.Summary.CheckovVersion != "" {
		s := probe.Summary
		return []RawReport{{Summary: &s, Results: &Results{}}}, nil
	}
	return nil, ErrNotCheckov
}

package checkov

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cauebraz/squeeze/internal/core"
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
	CountsOnly bool     `json:"-"`
	CheckType  string   `json:"check_type"`
	Results    *Results `json:"results"`
	Summary    *Summary `json:"summary"`
}

type Input struct {
	Reports  []RawReport
	Trailing []core.FlaggedLine
}

func Parse(input []byte) (Input, error) {
	reports, rest, err := parseReports(input)
	if err != nil {
		return Input{}, err
	}
	return Input{Reports: reports, Trailing: core.FlaggedLines(rest)}, nil
}

func parseReports(input []byte) ([]RawReport, []byte, error) {
	trimmed := bytes.TrimLeft(input, " \t\r\n")
	if len(trimmed) == 0 {
		return nil, nil, ErrNotCheckov
	}
	if trimmed[0] == '[' {
		var reports []RawReport
		rest, err := core.DecodeJSON(trimmed, &reports)
		if err != nil {
			return nil, nil, fmt.Errorf("decode: %w", err)
		}
		for i := range reports {
			if reports[i].Summary == nil {
				return nil, nil, ErrNotCheckov
			}
			if reports[i].Results == nil {
				reports[i].Results = &Results{}
			}
		}
		return reports, rest, nil
	}
	var probe struct {
		RawReport
		Summary
	}
	rest, err := core.DecodeJSON(trimmed, &probe)
	if err != nil {
		return nil, nil, fmt.Errorf("decode: %w", err)
	}
	if probe.Results != nil && probe.RawReport.Summary != nil {
		return []RawReport{probe.RawReport}, rest, nil
	}
	if probe.Summary.CheckovVersion != "" {
		s := probe.Summary
		return []RawReport{{Summary: &s, Results: &Results{}, CountsOnly: true}}, rest, nil
	}
	return nil, nil, ErrNotCheckov
}

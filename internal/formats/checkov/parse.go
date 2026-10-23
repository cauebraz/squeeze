package checkov

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/logscan"
)

var errNotCheckov = errors.New("input has no checkov report or summary")

type rawCheckResult struct {
	Result          string `json:"result"`
	SuppressComment string `json:"suppress_comment"`
}

type rawRecord struct {
	CheckID             string         `json:"check_id"`
	CheckName           string         `json:"check_name"`
	CheckResult         rawCheckResult `json:"check_result"`
	FilePath            string         `json:"file_path"`
	FileLineRange       []int          `json:"file_line_range"`
	Resource            string         `json:"resource"`
	Severity            *string        `json:"severity"`
	Guideline           *string        `json:"guideline"`
	CallerFilePath      *string        `json:"caller_file_path"`
	CallerFileLineRange []int          `json:"caller_file_line_range"`
}

type rawSummary struct {
	Passed         int    `json:"passed"`
	Failed         int    `json:"failed"`
	Skipped        int    `json:"skipped"`
	ParsingErrors  int    `json:"parsing_errors"`
	ResourceCount  int    `json:"resource_count"`
	CheckovVersion string `json:"checkov_version"`
}

type rawResults struct {
	PassedChecks  []rawRecord `json:"passed_checks"`
	FailedChecks  []rawRecord `json:"failed_checks"`
	SkippedChecks []rawRecord `json:"skipped_checks"`
	ParsingErrors []string    `json:"parsing_errors"`
}

type rawReport struct {
	CountsOnly bool        `json:"-"`
	CheckType  string      `json:"check_type"`
	Results    *rawResults `json:"results"`
	Summary    *rawSummary `json:"summary"`
}

type parsed struct {
	Reports  []rawReport
	Trailing []logscan.FlaggedLine
}

func parse(input []byte) (parsed, error) {
	reports, rest, err := parseReports(input)
	if err != nil {
		return parsed{}, err
	}
	return parsed{Reports: reports, Trailing: logscan.FlaggedLines(rest)}, nil
}

func parseReports(input []byte) ([]rawReport, []byte, error) {
	trimmed := bytes.TrimLeft(input, " \t\r\n")
	if len(trimmed) == 0 {
		return nil, nil, errNotCheckov
	}
	if trimmed[0] == '[' {
		var reports []rawReport
		rest, err := core.DecodeJSON(trimmed, &reports)
		if err != nil {
			return nil, nil, fmt.Errorf("decode: %w", err)
		}
		for i := range reports {
			if reports[i].Summary == nil {
				return nil, nil, errNotCheckov
			}
			if reports[i].Results == nil {
				reports[i].Results = &rawResults{}
			}
		}
		return reports, rest, nil
	}
	var report rawReport
	rest, err := core.DecodeJSON(trimmed, &report)
	if err != nil {
		return nil, nil, fmt.Errorf("decode: %w", err)
	}
	if report.Results != nil && report.Summary != nil {
		return []rawReport{report}, rest, nil
	}
	var counts rawSummary
	if _, err := core.DecodeJSON(trimmed, &counts); err != nil {
		return nil, nil, fmt.Errorf("decode: %w", err)
	}
	if counts.CheckovVersion != "" {
		return []rawReport{{Summary: &counts, Results: &rawResults{}, CountsOnly: true}}, rest, nil
	}
	return nil, nil, errNotCheckov
}

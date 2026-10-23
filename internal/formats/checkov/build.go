package checkov

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/sanitize"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const maxChecksInNotice = 5

var severities = core.SeverityOrder{"CRITICAL", "HIGH", "MEDIUM", "LOW", "INFO", "UNKNOWN"}

type check struct {
	Framework string `json:"framework"`
	CheckID   string `json:"check_id"`
	Name      string `json:"name"`
	Resource  string `json:"resource"`
	File      string `json:"file"`
	Start     int    `json:"start_line,omitempty"`
	End       int    `json:"end_line,omitempty"`
	Severity  string `json:"severity,omitempty"`
	Caller    string `json:"caller,omitempty"`
	Guideline string `json:"guideline,omitempty"`
	Comment   string `json:"suppress_comment,omitempty"`
}

type frameworkSummary struct {
	CheckType     string `json:"check_type"`
	Passed        int    `json:"passed"`
	Failed        int    `json:"failed"`
	Skipped       int    `json:"skipped"`
	ParsingErrors int    `json:"parsing_errors"`
	Resources     int    `json:"resource_count"`
}

type report struct {
	Tool           string              `json:"tool"`
	ToolVersion    string              `json:"tool_version"`
	Passed         int                 `json:"passed"`
	Failed         int                 `json:"failed"`
	Skipped        int                 `json:"skipped"`
	ParsingErrors  int                 `json:"parsing_errors"`
	Resources      int                 `json:"resource_count"`
	Frameworks     []frameworkSummary  `json:"frameworks"`
	BySeverity     map[string]int      `json:"failed_by_severity"`
	FailedChecks   []check             `json:"failed_checks"`
	ChecksOmitted  int                 `json:"failed_checks_omitted"`
	OmittedChecks  []core.Count        `json:"omitted_by_check_id"`
	SkippedChecks  []check             `json:"skipped_checks"`
	ParseErrorPath []string            `json:"parsing_error_files"`
	CountsOnly     bool                `json:"counts_only,omitempty"`
	Diagnostics    []tfdiag.Diagnostic `json:"diagnostics,omitempty"`
	perResource    map[string]int
	all            bool
}

func relative(p string) string {
	return strings.TrimPrefix(sanitize.Clean(p), "/")
}

func lines(r []int) (int, int) {
	if len(r) == 2 {
		return r[0], r[1]
	}
	if len(r) == 1 {
		return r[0], r[0]
	}
	return 0, 0
}

func convert(framework string, rec rawRecord) check {
	c := check{
		Framework: framework,
		CheckID:   sanitize.Clean(rec.CheckID),
		Name:      sanitize.Clean(rec.CheckName),
		Resource:  sanitize.Clean(rec.Resource),
		File:      relative(rec.FilePath),
		Comment:   sanitize.Clean(rec.CheckResult.SuppressComment),
	}
	if framework == "secrets" {
		c.Resource = "secret"
	}
	c.Start, c.End = lines(rec.FileLineRange)
	if rec.Severity != nil && *rec.Severity != "" {
		c.Severity = strings.ToUpper(sanitize.Clean(*rec.Severity))
		if !severities.Known(c.Severity) {
			c.Severity = "UNKNOWN"
		}
	}
	if rec.Guideline != nil {
		c.Guideline = sanitize.Clean(*rec.Guideline)
	}
	if rec.CallerFilePath != nil && *rec.CallerFilePath != "" {
		c.Caller = location(relative(*rec.CallerFilePath), rec.CallerFileLineRange)
	}
	return c
}

func location(file string, r []int) string {
	start, end := lines(r)
	switch {
	case start == 0:
		return file
	case start == end:
		return fmt.Sprintf("%s:%d", file, start)
	default:
		return fmt.Sprintf("%s:%d-%d", file, start, end)
	}
}

func build(in parsed, o options) report {
	r := report{
		Tool:           "checkov",
		Frameworks:     []frameworkSummary{},
		BySeverity:     map[string]int{},
		FailedChecks:   []check{},
		OmittedChecks:  []core.Count{},
		SkippedChecks:  []check{},
		ParseErrorPath: []string{},
		all:            o.all,
	}
	var failed []check
	r.Diagnostics = tfdiag.Normalize(tfdiag.FromLines(in.Trailing, ""))
	for _, rep := range in.Reports {
		s, res := rep.Summary, rep.Results
		if rep.CountsOnly {
			r.CountsOnly = true
		}
		if r.ToolVersion == "" {
			r.ToolVersion = sanitize.Clean(s.CheckovVersion)
		}
		ct := sanitize.Clean(rep.CheckType)
		failedHere := max(s.Failed, len(res.FailedChecks))
		skippedHere := max(s.Skipped, len(res.SkippedChecks))
		errorsHere := max(s.ParsingErrors, len(res.ParsingErrors))
		passedHere := max(s.Passed, len(res.PassedChecks))
		r.Passed += passedHere
		r.Failed += failedHere
		r.Skipped += skippedHere
		r.ParsingErrors += errorsHere
		r.Resources += s.ResourceCount
		if ct != "" {
			r.Frameworks = append(r.Frameworks, frameworkSummary{CheckType: ct, Passed: passedHere, Failed: failedHere, Skipped: skippedHere, ParsingErrors: errorsHere, Resources: s.ResourceCount})
		}
		for _, rec := range res.FailedChecks {
			c := convert(ct, rec)
			if c.Severity != "" {
				r.BySeverity[c.Severity]++
			}
			failed = append(failed, c)
		}
		for _, rec := range res.SkippedChecks {
			r.SkippedChecks = append(r.SkippedChecks, convert(ct, rec))
		}
		for _, p := range res.ParsingErrors {
			r.ParseErrorPath = append(r.ParseErrorPath, sanitize.Clean(p))
		}
	}
	sortGrouped(failed)
	r.perResource = map[string]int{}
	for _, c := range failed {
		r.perResource[c.key()]++
	}
	sortGrouped(r.SkippedChecks)
	slices.Sort(r.ParseErrorPath)
	kept, omitted := core.Limit(failed, o.maxChecks)
	r.FailedChecks = append(r.FailedChecks, kept...)
	r.ChecksOmitted = omitted
	if omitted > 0 {
		counts := map[string]int{}
		for _, c := range failed[len(kept):] {
			counts[c.CheckID]++
		}
		r.OmittedChecks = core.SortedCounts(counts)
	}
	return r
}

func (c check) key() string {
	return fmt.Sprintf("%s\x00%09d\x00%09d\x00%s\x00%s", c.File, c.Start, c.End, c.Resource, c.Caller)
}

func sortGrouped(checks []check) {
	type keyed struct {
		key   string
		rank  int
		check check
	}
	worst := map[string]int{}
	items := make([]keyed, len(checks))
	for i, c := range checks {
		k, r := c.key(), severities.Rank(c.Severity)
		items[i] = keyed{key: k, rank: r, check: c}
		if w, ok := worst[k]; !ok || r < w {
			worst[k] = r
		}
	}
	slices.SortStableFunc(items, func(a, b keyed) int {
		return cmp.Or(
			cmp.Compare(worst[a.key], worst[b.key]),
			strings.Compare(a.key, b.key),
			cmp.Compare(a.rank, b.rank),
			strings.Compare(a.check.CheckID, b.check.CheckID),
		)
	})
	for i, it := range items {
		checks[i] = it.check
	}
}

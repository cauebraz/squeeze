package checkov

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

const maxChecksInNotice = 5

var severityRank = map[string]int{"CRITICAL": 0, "HIGH": 1, "MEDIUM": 2, "LOW": 3, "INFO": 4}

type Check struct {
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

type Framework struct {
	CheckType     string `json:"check_type"`
	Passed        int    `json:"passed"`
	Failed        int    `json:"failed"`
	Skipped       int    `json:"skipped"`
	ParsingErrors int    `json:"parsing_errors"`
	Resources     int    `json:"resource_count"`
}

type CheckCount struct {
	CheckID string `json:"check_id"`
	Count   int    `json:"count"`
}

type Report struct {
	Tool           string         `json:"tool"`
	ToolVersion    string         `json:"tool_version"`
	Passed         int            `json:"passed"`
	Failed         int            `json:"failed"`
	Skipped        int            `json:"skipped"`
	ParsingErrors  int            `json:"parsing_errors"`
	Resources      int            `json:"resource_count"`
	Frameworks     []Framework    `json:"frameworks"`
	BySeverity     map[string]int `json:"failed_by_severity"`
	FailedChecks   []Check        `json:"failed_checks"`
	ChecksOmitted  int            `json:"failed_checks_omitted"`
	OmittedChecks  []CheckCount   `json:"omitted_check_ids"`
	SkippedChecks  []Check        `json:"skipped_checks"`
	ParseErrorPath []string       `json:"parsing_error_files"`
	PerResource    map[string]int `json:"-"`
}

func relative(p string) string {
	return strings.TrimPrefix(core.Clean(p), "/")
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

func convert(framework string, rec Record) Check {
	c := Check{
		Framework: framework,
		CheckID:   core.Clean(rec.CheckID),
		Name:      core.Clean(rec.CheckName),
		Resource:  core.Clean(rec.Resource),
		File:      relative(rec.FilePath),
		Comment:   core.Clean(rec.CheckResult.SuppressComment),
	}
	c.Start, c.End = lines(rec.FileLineRange)
	if rec.Severity != nil {
		c.Severity = strings.ToUpper(core.Clean(*rec.Severity))
	}
	if rec.Guideline != nil {
		c.Guideline = core.Clean(*rec.Guideline)
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

func rank(s string) int {
	if r, ok := severityRank[s]; ok {
		return r
	}
	return len(severityRank)
}

func Build(reports []RawReport, o Options) Report {
	r := Report{
		Tool:           "checkov",
		Frameworks:     []Framework{},
		BySeverity:     map[string]int{},
		FailedChecks:   []Check{},
		OmittedChecks:  []CheckCount{},
		SkippedChecks:  []Check{},
		ParseErrorPath: []string{},
	}
	var failed []Check
	for _, rep := range reports {
		s, res := rep.Summary, rep.Results
		if r.ToolVersion == "" {
			r.ToolVersion = core.Clean(s.CheckovVersion)
		}
		ct := core.Clean(rep.CheckType)
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
			r.Frameworks = append(r.Frameworks, Framework{CheckType: ct, Passed: passedHere, Failed: failedHere, Skipped: skippedHere, ParsingErrors: errorsHere, Resources: s.ResourceCount})
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
			r.ParseErrorPath = append(r.ParseErrorPath, core.Clean(p))
		}
	}
	sortGrouped(failed)
	r.PerResource = map[string]int{}
	for _, c := range failed {
		r.PerResource[c.key()]++
	}
	sortGrouped(r.SkippedChecks)
	sort.Strings(r.ParseErrorPath)
	kept, omitted := core.Limit(failed, o.MaxChecks)
	r.FailedChecks = append(r.FailedChecks, kept...)
	r.ChecksOmitted = omitted
	if omitted > 0 {
		counts := map[string]int{}
		for _, c := range failed[len(kept):] {
			counts[c.CheckID]++
		}
		for id, n := range counts {
			r.OmittedChecks = append(r.OmittedChecks, CheckCount{CheckID: id, Count: n})
		}
		sort.Slice(r.OmittedChecks, func(i, j int) bool {
			if r.OmittedChecks[i].Count != r.OmittedChecks[j].Count {
				return r.OmittedChecks[i].Count > r.OmittedChecks[j].Count
			}
			return r.OmittedChecks[i].CheckID < r.OmittedChecks[j].CheckID
		})
	}
	return r
}

func (c Check) key() string {
	return fmt.Sprintf("%s\x00%09d\x00%09d\x00%s\x00%s", c.File, c.Start, c.End, c.Resource, c.Caller)
}

func sortGrouped(checks []Check) {
	worst := map[string]int{}
	for _, c := range checks {
		k := c.key()
		if r, ok := worst[k]; !ok || rank(c.Severity) < r {
			worst[k] = rank(c.Severity)
		}
	}
	sort.SliceStable(checks, func(i, j int) bool {
		a, b := checks[i], checks[j]
		ka, kb := a.key(), b.key()
		if worst[ka] != worst[kb] {
			return worst[ka] < worst[kb]
		}
		if ka != kb {
			return ka < kb
		}
		if rank(a.Severity) != rank(b.Severity) {
			return rank(a.Severity) < rank(b.Severity)
		}
		return a.CheckID < b.CheckID
	})
}

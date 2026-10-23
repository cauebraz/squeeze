package precommit

import (
	"fmt"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/logscan"
)

type hookReport struct {
	hook
	Output       []string `json:"output"`
	LinesOmitted int      `json:"lines_omitted"`
}

type report struct {
	Tool         string       `json:"tool"`
	ToolVersion  string       `json:"tool_version"`
	Total        int          `json:"hooks_total"`
	Passed       int          `json:"passed"`
	Failed       int          `json:"failed"`
	Skipped      int          `json:"skipped"`
	Unfinished   int          `json:"unfinished"`
	Modified     int          `json:"modified"`
	Errors       []string     `json:"errors"`
	Warnings     []string     `json:"warnings"`
	Hooks        []hookReport `json:"hooks"`
	HooksOmitted int          `json:"hooks_omitted"`
	Other        []string     `json:"other"`
	OtherOmitted int          `json:"other_omitted"`
	ChangedFiles []string     `json:"changed_files"`
	Diff         []string     `json:"diff"`
	DiffOmitted  int          `json:"diff_omitted"`
}

func shown(h hook, all bool) bool {
	switch h.Status {
	case statusFailed, statusUnfinished:
		return true
	case statusPassed, statusSkipped:
		return all
	}
	return true
}

func build(run parsed, o options) report {
	r := report{
		Tool:         "pre-commit",
		Total:        len(run.hooks),
		Errors:       run.errors,
		Warnings:     run.warnings,
		Hooks:        []hookReport{},
		ChangedFiles: run.changedFiles,
		Diff:         []string{},
	}
	for _, h := range run.hooks {
		switch h.Status {
		case statusPassed:
			r.Passed++
		case statusFailed:
			r.Failed++
		case statusSkipped:
			r.Skipped++
		case statusUnfinished:
			r.Unfinished++
		}
		if h.Modified {
			r.Modified++
		}
		if !shown(h, o.All) {
			r.HooksOmitted++
			continue
		}
		hr := hookReport{hook: h}
		hr.Output, hr.LinesOmitted = cut(h.lines, o.MaxLines)
		r.Hooks = append(r.Hooks, hr)
	}
	r.Other, r.OtherOmitted = cut(run.other, o.MaxLines)
	if o.All {
		r.Diff = append(r.Diff, run.diff...)
	} else {
		r.DiffOmitted = len(run.diff)
	}
	return r
}

func cut(lines []string, limit int) ([]string, int) {
	out := []string{}
	if limit <= 0 || len(lines) <= limit {
		return append(out, lines...), 0
	}
	keep := make([]bool, len(lines))
	seen := map[string]bool{}
	flagged := false
	for i, l := range lines {
		if seen[l] || !(logscan.ErrorLine(l) || logscan.WarningLine(l)) {
			continue
		}
		flagged = true
		keep[i], seen[l] = true, true
		if i+1 < len(lines) && !seen[lines[i+1]] {
			keep[i+1], seen[lines[i+1]] = true, true
		}
	}
	if !flagged {
		for i := range lines[:limit] {
			keep[i] = true
		}
	}
	omitted, gap := 0, 0
	for i, l := range lines {
		if !keep[i] {
			gap++
			omitted++
			continue
		}
		if gap > 0 {
			out = append(out, gapLine(gap))
			gap = 0
		}
		out = append(out, l)
	}
	if gap > 0 {
		out = append(out, gapLine(gap))
	}
	return out, omitted
}

func gapLine(n int) string {
	return fmt.Sprintf("... %d %s cut (--max-lines 0)", n, core.Plural(n, "line", "lines"))
}

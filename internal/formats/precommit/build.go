package precommit

import (
	"fmt"

	"github.com/cauebraz/squeeze/internal/core"
)

type HookReport struct {
	Hook
	Output       []string `json:"output"`
	LinesOmitted int      `json:"lines_omitted"`
}

type Report struct {
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
	Hooks        []HookReport `json:"hooks"`
	HooksOmitted int          `json:"hooks_omitted"`
	Other        []string     `json:"other"`
	OtherOmitted int          `json:"other_omitted"`
	ChangedFiles []string     `json:"changed_files"`
	Diff         []string     `json:"diff"`
	DiffOmitted  int          `json:"diff_omitted"`
}

func cut(lines []string, max int) ([]string, int) {
	out := []string{}
	if max <= 0 || len(lines) <= max {
		return append(out, lines...), 0
	}
	keep := make([]bool, len(lines))
	seen := map[string]bool{}
	for i, l := range lines {
		if i < max {
			keep[i] = true
			seen[l] = true
			continue
		}
		if !seen[l] && (core.ErrorLine(l) || core.WarningLine(l)) {
			keep[i] = true
			seen[l] = true
			if i+1 < len(lines) && !seen[lines[i+1]] {
				keep[i+1] = true
				seen[lines[i+1]] = true
			}
		}
	}
	omitted, gap := 0, 0
	flush := func() {
		if gap > 0 {
			out = append(out, gapLine(gap))
			gap = 0
		}
	}
	for i, l := range lines {
		if !keep[i] {
			gap++
			omitted++
			continue
		}
		flush()
		out = append(out, l)
	}
	flush()
	return out, omitted
}

func gapLine(n int) string {
	return fmt.Sprintf("... %d %s cut (--max-lines 0)", n, core.Plural(n, "line", "lines"))
}

func shown(h Hook, all bool) bool {
	switch h.Status {
	case Failed, Unfinished:
		return true
	case Passed, Skipped:
		return all
	}
	return true
}

func Build(run Run, o Options) Report {
	r := Report{
		Tool:         "pre-commit",
		Total:        len(run.Hooks),
		Errors:       run.Errors,
		Warnings:     run.Warnings,
		Hooks:        []HookReport{},
		ChangedFiles: run.ChangedFiles,
		Diff:         []string{},
	}
	for _, h := range run.Hooks {
		switch h.Status {
		case Passed:
			r.Passed++
		case Failed:
			r.Failed++
		case Skipped:
			r.Skipped++
		case Unfinished:
			r.Unfinished++
		}
		if h.Modified {
			r.Modified++
		}
		if !shown(h, o.All) {
			r.HooksOmitted++
			continue
		}
		hr := HookReport{Hook: h}
		hr.Output, hr.LinesOmitted = cut(h.lines, o.MaxLines)
		r.Hooks = append(r.Hooks, hr)
	}
	r.Other, r.OtherOmitted = cut(run.Other, o.MaxLines)
	if o.All {
		r.Diff = append(r.Diff, run.Diff...)
	} else {
		r.DiffOmitted = len(run.Diff)
	}
	return r
}

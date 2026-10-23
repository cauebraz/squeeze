package ghactions

import (
	"regexp"
	"strings"

	"github.com/cauebraz/squeeze/internal/logscan"
	"github.com/cauebraz/squeeze/internal/sanitize"
)

var runnerVersion = regexp.MustCompile(`^Current runner version: '([^']+)'`)

const (
	setUpJob       = "set up job"
	postJobCleanup = "post job cleanup"
)

type step struct {
	Name         string   `json:"name"`
	Failed       bool     `json:"failed"`
	Errors       []string `json:"errors"`
	Warnings     []string `json:"warnings"`
	Output       []string `json:"output"`
	LinesOmitted int      `json:"lines_omitted"`
	lines        []string
}

type jobReport struct {
	Name   string `json:"name"`
	Failed bool   `json:"failed"`
	Steps  []step `json:"steps"`
}

type report struct {
	Tool        string      `json:"tool"`
	ToolVersion string      `json:"tool_version"`
	Errors      int         `json:"errors"`
	Warnings    int         `json:"warnings"`
	Jobs        []jobReport `json:"jobs"`
	OtherSteps  int         `json:"other_steps"`
}

func sections(lines []rawLine) ([]step, string) {
	steps := []step{{Name: setUpJob, Errors: []string{}, Warnings: []string{}}}
	version := ""
	header := false
	for _, l := range lines {
		msg := l.text
		cur := &steps[len(steps)-1]
		switch {
		case strings.HasPrefix(msg, "##[error]"):
			cur.Failed = true
			cur.Errors = append(cur.Errors, sanitize.Clean(strings.TrimPrefix(msg, "##[error]")))
		case strings.HasPrefix(msg, "##[warning]"):
			cur.Warnings = append(cur.Warnings, sanitize.Clean(strings.TrimPrefix(msg, "##[warning]")))
		case strings.HasPrefix(msg, "##[group]Run "):
			steps = append(steps, step{Name: sanitize.Clean(strings.TrimPrefix(msg, "##[group]Run ")), Errors: []string{}, Warnings: []string{}})
			header = true
		case header && strings.HasPrefix(msg, "##[endgroup]"):
			header = false
		case header:
		case strings.HasPrefix(msg, "Post job cleanup."):
			steps = append(steps, step{Name: postJobCleanup, Errors: []string{}, Warnings: []string{}})
		case strings.HasPrefix(msg, "##["):
		default:
			if m := runnerVersion.FindStringSubmatch(msg); m != nil && version == "" {
				version = sanitize.Clean(m[1])
			}
			cur.lines = append(cur.lines, sanitize.Line(msg))
		}
	}
	return steps, version
}

func build(jobs []job, o options) report {
	r := report{Tool: "github-actions", Jobs: []jobReport{}}
	for _, j := range jobs {
		steps, version := sections(j.lines)
		if r.ToolVersion == "" {
			r.ToolVersion = version
		}
		jr := jobReport{Name: sanitize.Clean(j.name), Steps: []step{}}
		for _, s := range steps {
			r.Errors += len(s.Errors)
			r.Warnings += len(s.Warnings)
			if s.Failed {
				jr.Failed = true
			}
			if !s.Failed && !o.All {
				if len(s.Warnings) > 0 {
					s.Output = []string{}
					jr.Steps = append(jr.Steps, s)
				} else if s.Name != setUpJob && s.Name != postJobCleanup {
					r.OtherSteps++
				}
				continue
			}
			s.Output, s.LinesOmitted = logscan.TailLines(s.lines, o.MaxLines, "--max-lines")
			jr.Steps = append(jr.Steps, s)
		}
		r.Jobs = append(r.Jobs, jr)
	}
	return r
}

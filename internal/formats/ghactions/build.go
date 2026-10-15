package ghactions

import (
	"regexp"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

var runnerVersion = regexp.MustCompile(`^Current runner version: '([^']+)'`)

type Step struct {
	Name         string   `json:"name"`
	Failed       bool     `json:"failed"`
	Errors       []string `json:"errors"`
	Warnings     []string `json:"warnings"`
	Output       []string `json:"output"`
	LinesOmitted int      `json:"lines_omitted"`
	lines        []string
}

type JobReport struct {
	Name   string `json:"name"`
	Failed bool   `json:"failed"`
	Steps  []Step `json:"steps"`
}

type Report struct {
	Tool        string      `json:"tool"`
	ToolVersion string      `json:"tool_version"`
	Errors      int         `json:"errors"`
	Warnings    int         `json:"warnings"`
	Jobs        []JobReport `json:"jobs"`
	OtherSteps  int         `json:"other_steps"`
}

func sections(lines []Line) ([]Step, string) {
	steps := []Step{{Name: "set up job", Errors: []string{}, Warnings: []string{}}}
	version := ""
	header := false
	for _, l := range lines {
		text := l.Text
		cur := &steps[len(steps)-1]
		switch {
		case strings.HasPrefix(text, "##[error]"):
			cur.Failed = true
			cur.Errors = append(cur.Errors, core.Clean(strings.TrimPrefix(text, "##[error]")))
		case strings.HasPrefix(text, "##[warning]"):
			cur.Warnings = append(cur.Warnings, core.Clean(strings.TrimPrefix(text, "##[warning]")))
		case strings.HasPrefix(text, "##[group]Run "):
			steps = append(steps, Step{Name: core.Clean(strings.TrimPrefix(text, "##[group]Run ")), Errors: []string{}, Warnings: []string{}})
			header = true
		case header && strings.HasPrefix(text, "##[endgroup]"):
			header = false
		case header:
		case strings.HasPrefix(text, "Post job cleanup."):
			steps = append(steps, Step{Name: "post job cleanup", Errors: []string{}, Warnings: []string{}})
		case strings.HasPrefix(text, "##["):
		default:
			if m := runnerVersion.FindStringSubmatch(text); m != nil && version == "" {
				version = core.Clean(m[1])
			}
			cur.lines = append(cur.lines, core.Line(text))
		}
	}
	return steps, version
}

func Build(jobs []Job, o Options) Report {
	r := Report{Tool: "github-actions", Jobs: []JobReport{}}
	for _, j := range jobs {
		steps, version := sections(j.Lines)
		if r.ToolVersion == "" {
			r.ToolVersion = version
		}
		jr := JobReport{Name: core.Clean(j.Name), Steps: []Step{}}
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
				} else if s.Name != "set up job" && s.Name != "post job cleanup" {
					r.OtherSteps++
				}
				continue
			}
			s.Output, s.LinesOmitted = core.TailLines(s.lines, o.MaxLines, "--max-lines")
			jr.Steps = append(jr.Steps, s)
		}
		r.Jobs = append(r.Jobs, jr)
	}
	return r
}

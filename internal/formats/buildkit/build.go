package buildkit

import "github.com/cauebraz/squeeze/internal/logscan"

type warning struct {
	Step    string   `json:"step"`
	Message string   `json:"message"`
	Detail  []string `json:"detail"`
	URL     string   `json:"url"`
}

type stepStatus string

const (
	statusFailed   stepStatus = "failed"
	statusCanceled stepStatus = "canceled"
	statusCached   stepStatus = "cached"
	statusDone     stepStatus = "done"
	statusRunning  stepStatus = "running"
)

type step struct {
	Name         string     `json:"name"`
	Status       stepStatus `json:"status"`
	Error        string     `json:"error"`
	Output       []string   `json:"output"`
	LinesOmitted int        `json:"lines_omitted"`
}

type report struct {
	Tool          string    `json:"tool"`
	ToolVersion   string    `json:"tool_version"`
	Progress      string    `json:"progress"`
	Builder       string    `json:"builder"`
	TotalSteps    int       `json:"total_steps"`
	CachedSteps   int       `json:"cached_steps"`
	FailedSteps   int       `json:"failed_steps"`
	CanceledSteps int       `json:"canceled_steps"`
	Warnings      []warning `json:"warnings"`
	Errors        []string  `json:"errors"`
	Location      string    `json:"location"`
	Steps         []step    `json:"steps"`
	OtherSteps    int       `json:"other_steps"`
}

func status(v *vertex) stepStatus {
	switch {
	case v.err != "":
		return statusFailed
	case v.canceled:
		return statusCanceled
	case v.cached:
		return statusCached
	case v.done:
		return statusDone
	}
	return statusRunning
}

func build(b *parsed, o options) report {
	r := report{
		Tool:     "buildkit",
		Progress: b.progress,
		Builder:  b.builder,
		Warnings: []warning{},
		Errors:   append([]string{}, b.errors...),
		Location: b.location,
		Steps:    []step{},
	}
	for _, w := range b.warnings {
		if v, ok := b.byKey[w.vertex]; ok {
			w.Step = v.name
		}
		r.Warnings = append(r.Warnings, w.warning)
	}
	for _, key := range b.order {
		v := b.byKey[key]
		s := step{Name: v.name, Status: status(v), Error: v.err}
		r.TotalSteps++
		switch s.Status {
		case statusFailed:
			r.FailedSteps++
		case statusCanceled:
			r.CanceledSteps++
		case statusCached:
			r.CachedSteps++
		case statusDone, statusRunning:
		}
		if !o.All && s.Status != statusFailed && s.Status != statusCanceled {
			r.OtherSteps++
			continue
		}
		s.Output, s.LinesOmitted = logscan.TailLines(v.logs, o.MaxLines, "--max-lines")
		r.Steps = append(r.Steps, s)
	}
	return r
}

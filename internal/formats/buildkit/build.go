package buildkit

import "github.com/cauebraz/squeeze/internal/core"

type Warning struct {
	Step    string   `json:"step"`
	Message string   `json:"message"`
	Detail  []string `json:"detail"`
	URL     string   `json:"url"`
}

type Step struct {
	Name         string   `json:"name"`
	Status       string   `json:"status"`
	Error        string   `json:"error"`
	Output       []string `json:"output"`
	LinesOmitted int      `json:"lines_omitted"`
}

type Report struct {
	Tool          string    `json:"tool"`
	ToolVersion   string    `json:"tool_version"`
	Progress      string    `json:"progress"`
	Builder       string    `json:"builder"`
	TotalSteps    int       `json:"total_steps"`
	CachedSteps   int       `json:"cached_steps"`
	FailedSteps   int       `json:"failed_steps"`
	CanceledSteps int       `json:"canceled_steps"`
	Warnings      []Warning `json:"warnings"`
	Errors        []string  `json:"errors"`
	Location      string    `json:"location"`
	Steps         []Step    `json:"steps"`
	OtherSteps    int       `json:"other_steps"`
}

func status(v *vertex) string {
	switch {
	case v.err != "":
		return "failed"
	case v.canceled:
		return "canceled"
	case v.cached:
		return "cached"
	case v.done:
		return "done"
	}
	return "running"
}

func Build(b *build, o Options) Report {
	r := Report{
		Tool:     "buildkit",
		Progress: b.progress,
		Builder:  b.builder,
		Warnings: []Warning{},
		Errors:   append([]string{}, b.errors...),
		Location: b.location,
		Steps:    []Step{},
	}
	for _, w := range b.warnings {
		if v, ok := b.byKey[w.vertex]; ok {
			w.Step = v.name
		}
		r.Warnings = append(r.Warnings, w.Warning)
	}
	for _, key := range b.order {
		v := b.byKey[key]
		s := Step{Name: v.name, Status: status(v), Error: v.err}
		r.TotalSteps++
		switch s.Status {
		case "failed":
			r.FailedSteps++
		case "canceled":
			r.CanceledSteps++
		case "cached":
			r.CachedSteps++
		}
		if !o.All && s.Status != "failed" && s.Status != "canceled" {
			r.OtherSteps++
			continue
		}
		s.Output, s.LinesOmitted = core.TailLines(v.logs, o.MaxLines, "--max-lines")
		r.Steps = append(r.Steps, s)
	}
	return r
}

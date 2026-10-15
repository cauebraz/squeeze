package cargo

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const maxReplacementBytes = 60

type Span struct {
	FileName                string  `json:"file_name"`
	LineStart               int     `json:"line_start"`
	ColumnStart             int     `json:"column_start"`
	IsPrimary               bool    `json:"is_primary"`
	Label                   *string `json:"label"`
	SuggestedReplacement    *string `json:"suggested_replacement"`
	SuggestionApplicability *string `json:"suggestion_applicability"`
	Text                    []struct {
		Text string `json:"text"`
	} `json:"text"`
}

type Raw struct {
	Message string `json:"message"`
	Code    *struct {
		Code string `json:"code"`
	} `json:"code"`
	Level    string `json:"level"`
	Spans    []Span `json:"spans"`
	Children []Raw  `json:"children"`
}

func severity(level string) (string, bool) {
	switch level {
	case "note", "help", "failure-note":
		return "", false
	}
	if strings.HasPrefix(level, "error") {
		return "error", true
	}
	return "warning", true
}

func primary(spans []Span) *Span {
	for i := range spans {
		if spans[i].IsPrimary {
			return &spans[i]
		}
	}
	if len(spans) > 0 {
		return &spans[0]
	}
	return nil
}

func noise(level, msg string) bool {
	return (level == "note" && strings.HasPrefix(msg, "`#[") && strings.HasSuffix(msg, "on by default")) ||
		(level == "help" && strings.HasPrefix(msg, "for further information visit "))
}

func convert(raw Raw) (Diagnostic, bool) {
	sev, ok := severity(raw.Level)
	if !ok {
		return Diagnostic{}, false
	}
	d := Diagnostic{Diagnostic: tfdiag.Diagnostic{Severity: sev, Summary: core.Clean(raw.Message)}}
	if raw.Code != nil {
		d.ID = core.Clean(raw.Code.Code)
		d.Address = d.ID
	}
	var parts []string
	if s := primary(raw.Spans); s != nil {
		d.File, d.Line, d.Column = core.Clean(s.FileName), s.LineStart, s.ColumnStart
		if len(s.Text) > 0 {
			d.Code = core.Clean(s.Text[0].Text)
		}
		if s.Label != nil && core.Clean(*s.Label) != "" {
			parts = append(parts, core.Clean(*s.Label))
		}
		for _, o := range raw.Spans {
			if o.IsPrimary || o.Label == nil || core.Clean(*o.Label) == "" {
				continue
			}
			where := fmt.Sprintf("line %d", o.LineStart)
			if o.FileName != s.FileName {
				where = fmt.Sprintf("%s:%d", o.FileName, o.LineStart)
			}
			parts = append(parts, fmt.Sprintf("%s (%s)", core.Clean(*o.Label), core.Clean(where)))
		}
	}
	d.Fixable = machineApplicable(raw.Spans)
	for _, c := range raw.Children {
		d.Fixable = d.Fixable || machineApplicable(c.Spans)
		if noise(c.Level, c.Message) {
			continue
		}
		part := core.Clean(c.Level + ": " + c.Message)
		if r := replacement(c.Spans); r != "" {
			part += ": `" + r + "`"
		}
		parts = append(parts, part)
	}
	d.Detail = strings.Join(parts, "; ")
	return d, true
}

func machineApplicable(spans []Span) bool {
	for _, s := range spans {
		if s.SuggestionApplicability != nil && *s.SuggestionApplicability == "MachineApplicable" {
			return true
		}
	}
	return false
}

func replacement(spans []Span) string {
	for _, s := range spans {
		if s.SuggestionApplicability != nil && *s.SuggestionApplicability == "HasPlaceholders" {
			continue
		}
		if r := s.SuggestedReplacement; r != nil && *r != "" && !strings.Contains(*r, "\n") && len(*r) <= maxReplacementBytes {
			return core.Clean(*r)
		}
	}
	return ""
}

var (
	arrowRe = regexp.MustCompile(`^\s*--> (.+):(\d+):(\d+)$`)
	noteRe  = regexp.MustCompile(`^\s*= (note|help): (.*)$`)
	childRe = regexp.MustCompile(`^(note|help): (.*)$`)
	codeRe  = regexp.MustCompile(`^\d+\s+\|(.*)$`)
	labelRe = regexp.MustCompile(`^\s*\|[\s|\-]*\^+ (.+)$`)
	lintRe  = regexp.MustCompile("^`#\\[(?:warn|deny|forbid)\\(([\\w:]+)\\)\\]`")
	exitRe  = regexp.MustCompile("^process didn't exit successfully: `[^`]*` ")
)

func (p *parser) human(i int) int {
	m := headerRe.FindStringSubmatch(p.lines[i])
	d := Diagnostic{Diagnostic: tfdiag.Diagnostic{Severity: m[1], Summary: core.Clean(m[3])}, ID: core.Clean(m[2])}
	var label string
	var parts []string
	j := i + 1
	for ; j < len(p.lines) && strings.TrimSpace(p.lines[j]) != "" && !headerRe.MatchString(p.lines[j]); j++ {
		l := p.lines[j]
		if a := arrowRe.FindStringSubmatch(l); a != nil && d.File == "" {
			d.File, d.Line, d.Column = core.Clean(a[1]), atoi(a[2]), atoi(a[3])
		} else if c := codeRe.FindStringSubmatch(l); c != nil && d.Code == "" {
			d.Code = core.Clean(c[1])
		} else if lb := labelRe.FindStringSubmatch(l); lb != nil && label == "" {
			label = core.Clean(lb[1])
		} else if n := noteRe.FindStringSubmatch(l); n != nil {
			if lint := lintRe.FindStringSubmatch(n[2]); lint != nil && d.ID == "" {
				d.ID = core.Clean(lint[1])
			}
			if !noise(n[1], n[2]) {
				parts = append(parts, core.Clean(n[1]+": "+n[2]))
			}
		} else if c := childRe.FindStringSubmatch(l); c != nil {
			parts = append(parts, core.Clean(c[1]+": "+c[2]))
		}
	}
	cause, next := p.cause(j)
	for _, c := range cause {
		if c == "--- stderr" || c == "--- stdout" || strings.HasPrefix(c, "note: run with `RUST_BACKTRACE") {
			continue
		}
		if pm := panicRe.FindStringSubmatch(c); pm != nil {
			if d.File == "" {
				d.File, d.Line, d.Column = core.Clean(pm[1]), atoi(pm[2]), atoi(pm[3])
			}
			continue
		}
		parts = append(parts, exitRe.ReplaceAllString(c, "process didn't exit successfully "))
	}
	if label != "" {
		parts = append([]string{label}, parts...)
	}
	d.Detail = strings.Join(parts, "; ")
	d.Address = d.ID
	p.run.Diagnostics = append(p.run.Diagnostics, d)
	return max(next, j)
}

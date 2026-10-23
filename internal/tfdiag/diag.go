package tfdiag

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

const (
	maxCodeBytes   = 120
	maxValueBytes  = 200
	maxDetailWidth = 100
	maxDetailLines = 3
)

type Pos struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type Range struct {
	Filename string `json:"filename"`
	Start    Pos    `json:"start"`
	End      Pos    `json:"end"`
}

type Snippet struct {
	Code                 string            `json:"code"`
	StartLine            int               `json:"start_line"`
	HighlightStartOffset int               `json:"highlight_start_offset"`
	HighlightEndOffset   int               `json:"highlight_end_offset"`
	Values               []RawValue        `json:"values"`
	TestAssertionExpr    *RawTestAssertion `json:"test_assertion_expr"`
}

type RawValue struct {
	Traversal string `json:"traversal"`
	Statement string `json:"statement"`
}

type RawTestAssertion struct {
	LHS     string `json:"lhs"`
	RHS     string `json:"rhs"`
	Warning string `json:"warning"`
}

type Raw struct {
	Severity string   `json:"severity"`
	Summary  string   `json:"summary"`
	Detail   string   `json:"detail"`
	Address  string   `json:"address"`
	Range    *Range   `json:"range"`
	Snippet  *Snippet `json:"snippet"`
}

type Diagnostic struct {
	Severity string   `json:"severity"`
	Summary  string   `json:"summary"`
	Detail   string   `json:"detail,omitempty"`
	Address  string   `json:"address,omitempty"`
	File     string   `json:"file,omitempty"`
	Line     int      `json:"line,omitempty"`
	Column   int      `json:"column,omitempty"`
	Code     string   `json:"code,omitempty"`
	Values   []string `json:"values,omitempty"`
	LHS      string   `json:"lhs,omitempty"`
	RHS      string   `json:"rhs,omitempty"`
	Mismatch string   `json:"assertion_warning,omitempty"`
	Count    int      `json:"count,omitempty"`
}

func Convert(r Raw) Diagnostic {
	d := Diagnostic{
		Severity: core.Clean(r.Severity),
		Summary:  core.Clean(r.Summary),
		Detail:   strings.TrimSpace(r.Detail),
		Address:  core.Clean(r.Address),
	}
	if d.Severity != "warning" {
		d.Severity = "error"
	}
	if r.Range != nil {
		d.File = core.Clean(r.Range.Filename)
		d.Line = r.Range.Start.Line
		d.Column = r.Range.Start.Column
	}
	if s := r.Snippet; s != nil {
		d.Code = codeLine(s)
		if a := s.TestAssertionExpr; a != nil {
			d.LHS = strings.TrimSpace(a.LHS)
			d.RHS = strings.TrimSpace(a.RHS)
			d.Mismatch = core.Clean(a.Warning)
		} else {
			for _, v := range s.Values {
				if line := core.Clean(v.Traversal + " " + v.Statement); line != "" {
					d.Values = append(d.Values, line)
				}
			}
		}
	}
	return d
}

func codeLine(s *Snippet) string {
	lines := strings.Split(s.Code, "\n")
	idx := 0
	if off := s.HighlightStartOffset; off > 0 && off <= len(s.Code) {
		idx = strings.Count(s.Code[:off], "\n")
	}
	if idx >= len(lines) {
		idx = len(lines) - 1
	}
	return truncate(core.Clean(lines[idx]), maxCodeBytes)
}

func Fold(ds []Diagnostic) []Diagnostic {
	out := make([]Diagnostic, 0, len(ds))
	index := map[string]int{}
	for _, d := range ds {
		d.Count = 0
		key := Key(d)
		if i, ok := index[key]; ok {
			out[i].Count = max(out[i].Count, 1) + 1
			continue
		}
		index[key] = len(out)
		out = append(out, d)
	}
	return out
}

func Key(d Diagnostic) string {
	d.Count = 0
	key, _ := json.Marshal(d)
	return string(key)
}

func severityRank(s string) int {
	switch s {
	case "error":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

func Sort(ds []Diagnostic) {
	slices.SortStableFunc(ds, func(a, b Diagnostic) int {
		return cmp.Or(
			cmp.Compare(severityRank(a.Severity), severityRank(b.Severity)),
			cmp.Compare(a.File, b.File),
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Column, b.Column),
			cmp.Compare(a.Summary, b.Summary),
			cmp.Compare(a.Address, b.Address),
		)
	})
}

func Normalize(ds []Diagnostic) []Diagnostic {
	out := Fold(ds)
	Sort(out)
	return out
}

func Limit(ds []Diagnostic, n int) ([]Diagnostic, int) {
	kept, _ := core.Limit(ds, n)
	omitted := 0
	for _, d := range ds[len(kept):] {
		omitted += max(d.Count, 1)
	}
	return kept, omitted
}

func Count(ds []Diagnostic) (errors, warnings int) {
	for _, d := range ds {
		n := max(d.Count, 1)
		switch d.Severity {
		case "error":
			errors += n
		case "warning":
			warnings += n
		}
	}
	return errors, warnings
}

type WriteOptions struct {
	Indent   string
	NoDetail bool
}

func Write(b *strings.Builder, d Diagnostic, o WriteOptions) {
	b.WriteString(o.Indent)
	b.WriteString(d.Severity)
	b.WriteString(": ")
	b.WriteString(d.Summary)
	if where := location(d); where != "" {
		b.WriteString(" (")
		b.WriteString(where)
		b.WriteByte(')')
	}
	if d.Count > 1 {
		fmt.Fprintf(b, " (x%d)", d.Count)
	}
	b.WriteByte('\n')
	if !o.NoDetail {
		for _, line := range wrap(d.Detail, maxDetailWidth, maxDetailLines) {
			b.WriteString(o.Indent)
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	if d.Code != "" {
		b.WriteString(o.Indent)
		b.WriteString("  > ")
		b.WriteString(d.Code)
		b.WriteByte('\n')
	}
	if o.NoDetail {
		return
	}
	for _, v := range d.Values {
		writeValue(b, o.Indent, "| ", v)
	}
	if d.LHS != "" || d.RHS != "" {
		writeValue(b, o.Indent, "lhs: ", d.LHS)
		writeValue(b, o.Indent, "rhs: ", d.RHS)
	}
	if d.Mismatch != "" {
		writeValue(b, o.Indent, "note: ", d.Mismatch)
	}
}

func writeValue(b *strings.Builder, indent, label, value string) {
	b.WriteString(indent)
	b.WriteString("  ")
	b.WriteString(label)
	b.WriteString(truncate(oneLine(value), maxValueBytes))
	b.WriteByte('\n')
}

func oneLine(s string) string {
	var buf bytes.Buffer
	if json.Compact(&buf, []byte(s)) == nil {
		return core.Clean(buf.String())
	}
	return strings.Join(strings.Fields(core.Clean(s)), " ")
}

func location(d Diagnostic) string {
	var parts []string
	if d.File != "" {
		pos := d.File
		if d.Line > 0 {
			pos += fmt.Sprintf(":%d", d.Line)
			if d.Column > 0 {
				pos += fmt.Sprintf(":%d", d.Column)
			}
		}
		parts = append(parts, pos)
	}
	if d.Address != "" {
		parts = append(parts, d.Address)
	}
	return strings.Join(parts, ", ")
}

func wrap(text string, width, maxLines int) []string {
	words := strings.Fields(core.Clean(text))
	var lines []string
	var cur strings.Builder
	for _, w := range words {
		if cur.Len() > 0 && cur.Len()+1+len(w) > width {
			lines = append(lines, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(w)
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] = cutWords(lines[maxLines-1], width-3) + "..."
	}
	return lines
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return cut(s, n-3) + "..."
}

func cutWords(s string, n int) string {
	if len(s) <= n {
		return s
	}
	short := cut(s, n)
	if i := strings.LastIndex(short, " "); i > 0 && s[len(short)] != ' ' {
		return strings.TrimRight(short[:i], " ")
	}
	return short
}

func cut(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(core.Truncate(s, n), " ")
}

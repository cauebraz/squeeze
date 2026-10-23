package generic

import (
	"fmt"
	"regexp"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
)

const minRun = 3

var digits = regexp.MustCompile(`[0-9]+`)

type report struct {
	Tool         string `json:"tool"`
	Lines        int    `json:"lines"`
	Printed      int    `json:"printed"`
	BlankDropped int    `json:"blank_dropped"`
	Folded       int    `json:"folded"`
	Cut          int    `json:"lines_omitted"`
	CutFlagged   int    `json:"flagged_lines_omitted"`
	LongLines    int    `json:"long_lines"`
	ErrorLines   int    `json:"error_lines"`
	WarningLines int    `json:"warning_lines"`
	JSONIndented bool   `json:"json_indented"`
	out          []string
}

type entry struct {
	text   string
	marker bool
}

type group struct {
	entries []entry
	content int
	lines   int
	flags   int
	errors  int
	flagged bool
}

func build(p parsed, o options) report {
	r := report{Tool: "text", Lines: len(p.lines), JSONIndented: p.jsonIndented}
	for _, l := range p.lines {
		if l.isError {
			r.ErrorLines++
		}
		if l.warning {
			r.WarningLines++
		}
	}
	cleaned := dropBlanks(p.lines, &r)
	groups := fold(cleaned, o.NoFold, &r)
	groups = cut(groups, o.MaxLines, &r)
	r.out, r.Printed, r.LongLines = flatten(groups, o.MaxLineBytes)
	return r
}

func dropBlanks(lines []line, r *report) []line {
	out := make([]line, 0, len(lines))
	for _, l := range lines {
		if l.text == "" && (len(out) == 0 || out[len(out)-1].text == "") {
			r.BlankDropped++
			continue
		}
		out = append(out, l)
	}
	if len(out) > 0 && out[len(out)-1].text == "" {
		out = out[:len(out)-1]
		r.BlankDropped++
	}
	return out
}

func similar(a, b line) bool {
	if a.text == b.text {
		return true
	}
	if a.text == "" || b.text == "" || a.isError || b.isError {
		return false
	}
	return digits.ReplaceAllString(a.text, "0") == digits.ReplaceAllString(b.text, "0")
}

func single(l line) group {
	g := group{entries: []entry{{text: l.text}}, content: 1, lines: 1}
	if l.flagged() {
		g.flagged, g.flags = true, 1
	}
	if l.isError {
		g.errors = 1
	}
	return g
}

func fold(lines []line, off bool, r *report) []group {
	out := make([]group, 0, len(lines))
	for i := 0; i < len(lines); {
		j := i + 1
		identical := true
		for !off && j < len(lines) && similar(lines[i], lines[j]) {
			identical = identical && lines[i].text == lines[j].text
			j++
		}
		n := j - i
		if n < minRun {
			for k := i; k < j; k++ {
				out = append(out, single(lines[k]))
			}
			i = j
			continue
		}
		g := single(lines[i])
		g.lines = n
		if identical {
			g.entries = append(g.entries, entry{text: fmt.Sprintf("... %d more identical %s (--no-fold)", n-1, core.Plural(n-1, "line", "lines")), marker: true})
			g.flags *= n
			g.errors *= n
			r.Folded += n - 1
		} else {
			hidden := 0
			g.flags = 0
			for k := i; k < j; k++ {
				if lines[k].flagged() {
					g.flags++
					if k > i && k < j-1 {
						hidden++
					}
				}
			}
			g.flagged = g.flags > 0
			notice := fmt.Sprintf("... %d similar %s (--no-fold)", n-2, core.Plural(n-2, "line", "lines"))
			if hidden > 0 {
				notice = fmt.Sprintf("... %d similar %s, %d with warning words (--no-fold)", n-2, core.Plural(n-2, "line", "lines"), hidden)
			}
			g.entries = append(g.entries, entry{text: notice, marker: true}, entry{text: lines[j-1].text})
			g.content = 2
			r.Folded += n - 2
		}
		out = append(out, g)
		i = j
	}
	return out
}

func cut(groups []group, limit int, r *report) []group {
	total := 0
	for _, g := range groups {
		total += g.content
	}
	if limit <= 0 || total <= limit {
		return groups
	}
	head, used := 0, 0
	for head < len(groups) && used+groups[head].content <= limit/4 {
		used += groups[head].content
		head++
	}
	tail, usedTail := len(groups), 0
	for tail > head && usedTail+groups[tail-1].content <= limit/2 {
		tail--
		usedTail += groups[tail].content
	}
	budget := limit - used - usedTail
	out := slices.Clone(groups[:head])
	var gap group
	flush := func() {
		if gap.lines == 0 {
			return
		}
		notice := fmt.Sprintf("... %d %s cut (--max-lines 0)", gap.lines, core.Plural(gap.lines, "line", "lines"))
		if gap.flags > 0 {
			notice = fmt.Sprintf("... %d %s cut, %d with error or warning words (--max-lines 0)", gap.lines, core.Plural(gap.lines, "line", "lines"), gap.flags)
		}
		out = append(out, group{entries: []entry{{text: notice, marker: true}}})
		r.Cut += gap.lines
		r.CutFlagged += gap.flags
		gap = group{}
	}
	middle := groups[head:tail]
	keep := make([]bool, len(middle))
	take := func(i int) {
		if !keep[i] && middle[i].content <= budget {
			budget -= middle[i].content
			keep[i] = true
		}
	}
	for i, g := range middle {
		if g.errors > 0 {
			take(i)
		}
	}
	for i, g := range middle {
		if g.flagged {
			take(i)
		}
	}
	for i := 1; i < len(middle); i++ {
		if keep[i] && middle[i].errors > 0 {
			take(i - 1)
		}
	}
	for i, g := range middle {
		if keep[i] {
			flush()
			out = append(out, g)
			continue
		}
		r.Folded -= g.lines - g.content
		gap.lines += g.lines
		gap.flags += g.flags
	}
	flush()
	return append(out, groups[tail:]...)
}

func flatten(groups []group, maxBytes int) ([]string, int, int) {
	var out []string
	printed, long := 0, 0
	for _, g := range groups {
		for _, e := range g.entries {
			s := e.text
			if !e.marker {
				printed++
				if maxBytes > 0 && len(s) > maxBytes {
					short := core.Truncate(s, maxBytes)
					s = fmt.Sprintf("%s ... %d more bytes (--max-line-bytes 0)", short, len(s)-len(short))
					long++
				}
			}
			out = append(out, s)
		}
	}
	return out, printed, long
}

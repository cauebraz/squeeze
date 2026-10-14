package generic

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

const minRun = 3

var (
	errorWords   = regexp.MustCompile(`(?i)\b(errors?|fail(s|ed|ures?)?|fatal|panic(s|ked)?|exceptions?|tracebacks?)\b`)
	warningWords = regexp.MustCompile(`(?i)\b(warn(s|ed|ings?)?)\b`)
	negated      = regexp.MustCompile(`(?i)(^|[^\w])(0|zero|no|without|non)[\s-]*$`)
	zeroAfter    = regexp.MustCompile(`^\s*[:=]\s*0\b`)
)

func flagged(line string, words *regexp.Regexp) bool {
	for _, m := range words.FindAllStringIndex(line, -1) {
		if negated.MatchString(line[:m[0]]) || zeroAfter.MatchString(line[m[1]:]) {
			continue
		}
		return true
	}
	return false
}

func isError(line string) bool { return flagged(line, errorWords) }

func isWarning(line string) bool { return flagged(line, warningWords) }

func isFlagged(line string) bool { return isError(line) || isWarning(line) }

var digits = regexp.MustCompile(`[0-9]+`)

type Report struct {
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
	flagged bool
}

func Build(input []byte, o Options) (Report, string) {
	r := Report{Tool: "text"}
	lines := splitLines(string(input))
	r.Lines = len(lines)
	for _, l := range lines {
		if isError(l) {
			r.ErrorLines++
		}
		if isWarning(l) {
			r.WarningLines++
		}
	}
	cleaned := dropBlanks(lines, &r)
	groups := fold(cleaned, o.NoFold, &r)
	groups = cut(groups, o.MaxLines, &r)
	return r, render(groups, o.MaxLineBytes, &r)
}

func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	raw := strings.Split(s, "\n")
	out := make([]string, len(raw))
	for i, l := range raw {
		frames := strings.Split(strings.TrimSuffix(l, "\r"), "\r")
		for j := len(frames) - 1; j >= 0; j-- {
			if out[i] = core.Line(frames[j]); strings.TrimSpace(out[i]) != "" {
				break
			}
		}
		if strings.TrimSpace(out[i]) == "" {
			out[i] = ""
		}
	}
	return out
}

func dropBlanks(lines []string, r *Report) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if l == "" && (len(out) == 0 || out[len(out)-1] == "") {
			r.BlankDropped++
			continue
		}
		out = append(out, l)
	}
	if len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
		r.BlankDropped++
	}
	return out
}

func similar(a, b string) bool {
	if a == b {
		return true
	}
	if a == "" || b == "" || isFlagged(a) || isFlagged(b) {
		return false
	}
	return digits.ReplaceAllString(a, "0") == digits.ReplaceAllString(b, "0")
}

func single(text string) group {
	g := group{entries: []entry{{text: text}}, content: 1, lines: 1}
	if isFlagged(text) {
		g.flagged, g.flags = true, 1
	}
	return g
}

func fold(lines []string, off bool, r *Report) []group {
	out := make([]group, 0, len(lines))
	for i := 0; i < len(lines); {
		j := i + 1
		identical := true
		for !off && j < len(lines) && similar(lines[i], lines[j]) {
			identical = identical && lines[i] == lines[j]
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
			r.Folded += n - 1
		} else {
			g.entries = append(g.entries,
				entry{text: fmt.Sprintf("... %d similar %s (--no-fold)", n-2, core.Plural(n-2, "line", "lines")), marker: true},
				entry{text: lines[j-1]})
			g.content = 2
			r.Folded += n - 2
		}
		out = append(out, g)
		i = j
	}
	return out
}

func cut(groups []group, max int, r *Report) []group {
	total := 0
	for _, g := range groups {
		total += g.content
	}
	if max <= 0 || total <= max {
		return groups
	}
	head, used := 0, 0
	for head < len(groups) && used+groups[head].content <= max/4 {
		used += groups[head].content
		head++
	}
	tail, usedTail := len(groups), 0
	for tail > head && usedTail+groups[tail-1].content <= max/2 {
		tail--
		usedTail += groups[tail].content
	}
	budget := max - used - usedTail
	out := append([]group(nil), groups[:head]...)
	var gap group
	flush := func() {
		if gap.lines == 0 {
			return
		}
		text := fmt.Sprintf("... %d %s cut (--max-lines 0)", gap.lines, core.Plural(gap.lines, "line", "lines"))
		if gap.flags > 0 {
			text = fmt.Sprintf("... %d %s cut, %d with error or warning words (--max-lines 0)", gap.lines, core.Plural(gap.lines, "line", "lines"), gap.flags)
		}
		out = append(out, group{entries: []entry{{text: text, marker: true}}})
		r.Cut += gap.lines
		r.CutFlagged += gap.flags
		gap = group{}
	}
	for _, g := range groups[head:tail] {
		if g.flagged && g.content <= budget {
			budget -= g.content
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

func render(groups []group, maxBytes int, r *Report) string {
	if len(groups) == 0 {
		return "(no printable text)\n"
	}
	var b strings.Builder
	for _, g := range groups {
		for _, e := range g.entries {
			text := e.text
			if !e.marker {
				r.Printed++
				if maxBytes > 0 && len(text) > maxBytes {
					short := core.Truncate(text, maxBytes)
					text = fmt.Sprintf("%s ... %d more bytes (--max-line-bytes 0)", short, len(text)-len(short))
					r.LongLines++
				}
			}
			b.WriteString(text)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

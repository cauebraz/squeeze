package generic

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

const minRun = 3

var flagWords = regexp.MustCompile(`(?i)\b(error|errors|fail|failed|failure|failures|fatal|panic|exception|traceback|warn|warning|warnings)\b`)

var digits = regexp.MustCompile(`[0-9]+`)

type Report struct {
	Lines        int `json:"lines"`
	Printed      int `json:"printed"`
	BlankDropped int `json:"blank_dropped"`
	Folded       int `json:"folded"`
	Cut          int `json:"cut"`
	CutFlagged   int `json:"cut_flagged"`
	LongLines    int `json:"long_lines"`
}

type entry struct {
	text    string
	flagged bool
	marker  bool
	lines   int
	flags   int
}

func Build(input []byte, o Options) (Report, string) {
	var r Report
	lines := splitLines(string(input))
	r.Lines = len(lines)
	cleaned := dropBlanks(lines, &r)
	entries := fold(cleaned, o.NoFold, &r)
	entries = cut(entries, o.MaxLines, &r)
	return r, render(entries, o.MaxLineBytes, &r)
}

func splitLines(s string) []string {
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	raw := strings.Split(s, "\n")
	out := make([]string, len(raw))
	for i, l := range raw {
		l = strings.TrimSuffix(l, "\r")
		if j := strings.LastIndexByte(l, '\r'); j >= 0 {
			if rest := l[j+1:]; strings.TrimSpace(rest) != "" {
				l = rest
			} else {
				l = lastSegment(l)
			}
		}
		out[i] = core.Line(l)
	}
	return out
}

func lastSegment(l string) string {
	parts := strings.Split(l, "\r")
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.TrimSpace(parts[i]) != "" {
			return parts[i]
		}
	}
	return ""
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
	if a == "" || b == "" || flagWords.MatchString(a) || flagWords.MatchString(b) {
		return false
	}
	return digits.ReplaceAllString(a, "0") == digits.ReplaceAllString(b, "0")
}

func line(text string) entry {
	e := entry{text: text, lines: 1}
	if flagWords.MatchString(text) {
		e.flagged, e.flags = true, 1
	}
	return e
}

func fold(lines []string, off bool, r *Report) []entry {
	out := make([]entry, 0, len(lines))
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
				out = append(out, line(lines[k]))
			}
			i = j
			continue
		}
		first := line(lines[i])
		if identical {
			marker := entry{text: fmt.Sprintf("... %d more identical %s (--no-fold)", n-1, plural(n-1, "line", "lines")), marker: true, lines: n - 1, flags: first.flags * (n - 1)}
			out = append(out, first, marker)
			r.Folded += n - 1
		} else {
			marker := entry{text: fmt.Sprintf("... %d similar %s (--no-fold)", n-2, plural(n-2, "line", "lines")), marker: true, lines: n - 2}
			out = append(out, first, marker, line(lines[j-1]))
			r.Folded += n - 2
		}
		i = j
	}
	return out
}

func cut(entries []entry, max int, r *Report) []entry {
	if max <= 0 || len(entries) <= max {
		return entries
	}
	head, tail := max/4, max/2
	budget := max - head - tail
	middle := entries[head : len(entries)-tail]
	keep := make([]bool, len(middle))
	for i, e := range middle {
		if budget == 0 {
			break
		}
		if e.flagged {
			keep[i] = true
			budget--
		}
	}
	out := append([]entry(nil), entries[:head]...)
	var gap entry
	flush := func() {
		if gap.lines == 0 {
			return
		}
		text := fmt.Sprintf("... %d %s cut (--max-lines 0)", gap.lines, plural(gap.lines, "line", "lines"))
		if gap.flags > 0 {
			text = fmt.Sprintf("... %d %s cut, %d with error or warning words (--max-lines 0)", gap.lines, plural(gap.lines, "line", "lines"), gap.flags)
		}
		out = append(out, entry{text: text, marker: true})
		r.Cut += gap.lines
		r.CutFlagged += gap.flags
		gap = entry{}
	}
	for i, e := range middle {
		if keep[i] {
			flush()
			out = append(out, e)
			continue
		}
		if e.marker {
			r.Folded -= e.lines
		}
		gap.lines += e.lines
		gap.flags += e.flags
	}
	flush()
	return append(out, entries[len(entries)-tail:]...)
}

func render(entries []entry, maxBytes int, r *Report) string {
	if len(entries) == 0 {
		return "(no printable text)\n"
	}
	var b strings.Builder
	for _, e := range entries {
		text := e.text
		if !e.marker {
			r.Printed++
			if maxBytes > 0 && len(text) > maxBytes {
				short := truncate(text, maxBytes)
				text = fmt.Sprintf("%s ... [%d more bytes, --max-line-bytes 0]", short, len(text)-len(short))
				r.LongLines++
			}
		}
		b.WriteString(text)
		b.WriteByte('\n')
	}
	return b.String()
}

func truncate(s string, n int) string {
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

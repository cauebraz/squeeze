package ghactions

import (
	"bytes"
	"errors"
	"regexp"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

var errNoLog = errors.New("no GitHub Actions log lines in the input")

var (
	stamp   = regexp.MustCompile(`^\x{FEFF}?\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d+)?Z ?`)
	columns = regexp.MustCompile(`^([^\t]+)\t[^\t]*\t(\x{FEFF}?\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d+)?Z ?.*)$`)
)

const detected = 3

type rawLine struct {
	job  string
	text string
}

type job struct {
	name  string
	lines []rawLine
}

func detect(head []byte) bool {
	seen := 0
	for _, l := range bytes.Split(head, []byte("\n")) {
		l = bytes.TrimRight(l, "\r")
		if len(bytes.TrimSpace(l)) == 0 {
			continue
		}
		if !columns.Match(l) {
			return false
		}
		seen++
		if seen == detected {
			return true
		}
	}
	return seen > 0
}

func parse(input []byte) ([]job, error) {
	var jobs []job
	index := map[string]int{}
	found := false
	for raw := range core.SplitLines(string(input)) {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		l := rawLine{text: raw}
		if m := columns.FindStringSubmatch(raw); m != nil {
			l = rawLine{job: m[1], text: m[2]}
		}
		if loc := stamp.FindStringIndex(l.text); loc != nil {
			l.text = l.text[loc[1]:]
			found = true
		}
		i, ok := index[l.job]
		if !ok {
			i = len(jobs)
			index[l.job] = i
			jobs = append(jobs, job{name: l.job})
		}
		jobs[i].lines = append(jobs[i].lines, l)
	}
	if !found {
		return nil, errNoLog
	}
	return jobs, nil
}

package ghactions

import (
	"bytes"
	"errors"
	"regexp"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

var ErrNoLog = errors.New("no GitHub Actions log lines in the input")

var (
	stamp    = regexp.MustCompile(`^\x{FEFF}?\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d+)?Z ?`)
	columns  = regexp.MustCompile(`^([^\t]+)\t([^\t]*)\t(\x{FEFF}?\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d+)?Z ?.*)$`)
	detected = 3
)

type Line struct {
	Job  string
	Step string
	Text string
}

type Job struct {
	Name  string
	Lines []Line
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

func Parse(input []byte) ([]Job, error) {
	var jobs []Job
	index := map[string]int{}
	found := false
	for raw := range core.SplitLines(string(input)) {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		l := Line{Text: raw}
		if m := columns.FindStringSubmatch(raw); m != nil {
			l = Line{Job: m[1], Step: m[2], Text: m[3]}
		}
		if loc := stamp.FindStringIndex(l.Text); loc != nil {
			l.Text = l.Text[loc[1]:]
			found = true
		}
		i, ok := index[l.Job]
		if !ok {
			i = len(jobs)
			index[l.Job] = i
			jobs = append(jobs, Job{Name: l.Job})
		}
		jobs[i].Lines = append(jobs[i].Lines, l)
	}
	if !found {
		return nil, ErrNoLog
	}
	return jobs, nil
}

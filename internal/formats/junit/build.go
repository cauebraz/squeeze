package junit

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/sanitize"
)

const maxMessage = 300

var ruleLine = regexp.MustCompile(`^\s*[-=]{3,}( .* [-=]{3,})?\s*$`)

type attributes struct {
	Tests    *int `json:"tests"`
	Failures *int `json:"failures"`
	Errors   *int `json:"errors"`
	Skipped  *int `json:"skipped"`
}

type mismatch struct {
	Count     string `json:"count"`
	Counted   int    `json:"counted"`
	Attribute int    `json:"attribute"`
}

type caseKind string

const (
	kindFailure caseKind = "failure"
	kindError   caseKind = "error"
)

type failedCase struct {
	Kind      caseKind   `json:"kind"`
	Suite     string     `json:"suite"`
	Classname string     `json:"classname"`
	Name      string     `json:"name"`
	File      string     `json:"file"`
	Line      int        `json:"line"`
	Type      string     `json:"type"`
	Message   string     `json:"message"`
	Text      core.Block `json:"text"`
	Stdout    core.Block `json:"stdout"`
	Stderr    core.Block `json:"stderr"`
}

type suiteOutput struct {
	Suite  string     `json:"suite"`
	Stdout core.Block `json:"stdout"`
	Stderr core.Block `json:"stderr"`
}

type report struct {
	Tool         string        `json:"tool"`
	ToolVersion  string        `json:"tool_version"`
	Suites       int           `json:"suites"`
	Tests        int           `json:"tests"`
	Passed       int           `json:"passed"`
	Failures     int           `json:"failures"`
	Errors       int           `json:"errors"`
	Skipped      int           `json:"skipped"`
	Flaky        int           `json:"flaky"`
	Attributes   attributes    `json:"attributes"`
	Mismatches   []mismatch    `json:"mismatches"`
	Cases        []failedCase  `json:"cases"`
	TestsOmitted int           `json:"tests_omitted"`
	SuiteOutput  []suiteOutput `json:"suite_output"`
}

type builder struct {
	o        options
	r        *report
	elements int
	seen     map[string]bool
	notPass  map[string]bool
	cases    []failedCase
}

func number(s string) *int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return nil
	}
	return &n
}

func total(root *rawSuite, get func(rawSuite) string) *int {
	if n := number(get(*root)); n != nil || root.XMLName.Local == "testsuite" {
		return n
	}
	var sum *int
	for _, s := range root.Suites {
		if n := number(get(s)); n != nil {
			if sum == nil {
				sum = new(int)
			}
			*sum += *n
		}
	}
	return sum
}

func useful(text string) []string {
	out := []string{}
	for l := range core.SplitLines(text) {
		l = sanitize.Line(l)
		if strings.TrimSpace(l) == "" || ruleLine.MatchString(l) {
			continue
		}
		out = append(out, l)
	}
	return out
}

func (b *builder) block(texts []string) core.Block {
	return core.HeadBlock(useful(strings.Join(texts, "\n")), b.o.MaxOutput)
}

func (b *builder) failed(kind caseKind, suite string, c rawCase, res rawResult) failedCase {
	f := failedCase{Kind: kind, Suite: suite, Classname: sanitize.Clean(c.Classname), Name: sanitize.Clean(c.Name), File: sanitize.Clean(c.File), Type: sanitize.Clean(res.Type), Message: sanitize.Clean(res.Message)}
	if n := number(c.Line); n != nil && *n > 0 {
		f.Line = *n
	}
	lines := useful(res.Text)
	if f.Message == "" && len(lines) > 0 {
		f.Message, lines = sanitize.Clean(lines[0]), lines[1:]
	}
	if f.Message == "" {
		f.Message = "no message"
	}
	if len(lines) > 0 && sanitize.Clean(lines[0]) == f.Message {
		lines = lines[1:]
	}
	f.Text = core.HeadBlock(lines, b.o.MaxOutput)
	f.Stdout, f.Stderr = b.block(c.SystemOut), b.block(c.SystemErr)
	return f
}

func (b *builder) walk(s rawSuite, counted bool) {
	index := b.elements
	b.elements++
	if counted {
		b.r.Suites++
	}
	name := sanitize.Clean(s.Name)
	failing := false
	for _, c := range s.Cases {
		key := strconv.Itoa(index) + "\x00" + c.Classname + "\x00" + c.Name
		if !b.seen[key] {
			b.seen[key] = true
			b.r.Tests++
		}
		bad := len(c.Failures) > 0 || len(c.Errors) > 0
		switch {
		case bad:
			failing = true
		case len(c.Skipped) > 0:
			b.r.Skipped++
		case len(c.FlakyFailure) > 0 || len(c.FlakyError) > 0:
			b.r.Flaky++
		}
		if bad || len(c.Skipped) > 0 {
			b.notPass[key] = true
		}
		if len(c.Failures) > 0 {
			b.r.Failures++
		}
		if len(c.Errors) > 0 {
			b.r.Errors++
		}
		for _, res := range c.Failures {
			b.cases = append(b.cases, b.failed(kindFailure, name, c, res))
		}
		for _, res := range c.Errors {
			b.cases = append(b.cases, b.failed(kindError, name, c, res))
		}
	}
	if failing {
		out := suiteOutput{Suite: name, Stdout: b.block(s.SystemOut), Stderr: b.block(s.SystemErr)}
		if len(out.Stdout.Lines) > 0 || len(out.Stderr.Lines) > 0 {
			b.r.SuiteOutput = append(b.r.SuiteOutput, out)
		}
	}
	for _, child := range s.Suites {
		b.walk(child, true)
	}
}

func build(root *rawSuite, o options) report {
	r := report{Tool: "junit", Mismatches: []mismatch{}, Cases: []failedCase{}, SuiteOutput: []suiteOutput{}}
	b := &builder{o: o, r: &r, seen: map[string]bool{}, notPass: map[string]bool{}}
	b.walk(*root, root.XMLName.Local == "testsuite")
	r.Passed = r.Tests - len(b.notPass)
	r.Attributes = attributes{
		Tests:    total(root, func(s rawSuite) string { return s.Tests }),
		Failures: total(root, func(s rawSuite) string { return s.Failures }),
		Errors:   total(root, func(s rawSuite) string { return s.Errors }),
		Skipped:  total(root, func(s rawSuite) string { return s.Skipped }),
	}
	for _, m := range []struct {
		name    string
		counted int
		attr    *int
	}{{"tests", r.Tests, r.Attributes.Tests}, {"failures", r.Failures, r.Attributes.Failures}, {"errors", r.Errors, r.Attributes.Errors}, {"skipped", r.Skipped, r.Attributes.Skipped}} {
		if m.attr != nil && *m.attr != m.counted {
			r.Mismatches = append(r.Mismatches, mismatch{Count: m.name, Counted: m.counted, Attribute: *m.attr})
		}
	}
	kept, omitted := core.Limit(b.cases, o.MaxTests)
	r.Cases = append(r.Cases, kept...)
	r.TestsOmitted = omitted
	return r
}

func value(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

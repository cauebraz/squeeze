package junit

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

const maxMessage = 300

var ruleLine = regexp.MustCompile(`^\s*[-=]{3,}( .* [-=]{3,})?\s*$`)

type Attributes struct {
	Tests    *int `json:"tests"`
	Failures *int `json:"failures"`
	Errors   *int `json:"errors"`
	Skipped  *int `json:"skipped"`
}

type Mismatch struct {
	Count     string `json:"count"`
	Counted   int    `json:"counted"`
	Attribute int    `json:"attribute"`
}

type Block struct {
	Lines        []string `json:"lines"`
	LinesOmitted int      `json:"lines_omitted"`
}

type FailedCase struct {
	Kind      string `json:"kind"`
	Suite     string `json:"suite"`
	Classname string `json:"classname"`
	Name      string `json:"name"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	Text      Block  `json:"text"`
	Stdout    Block  `json:"stdout"`
	Stderr    Block  `json:"stderr"`
}

type SuiteOutput struct {
	Suite  string `json:"suite"`
	Stdout Block  `json:"stdout"`
	Stderr Block  `json:"stderr"`
}

type Report struct {
	Tool         string        `json:"tool"`
	ToolVersion  string        `json:"tool_version"`
	Suites       int           `json:"suites"`
	Tests        int           `json:"tests"`
	Passed       int           `json:"passed"`
	Failures     int           `json:"failures"`
	Errors       int           `json:"errors"`
	Skipped      int           `json:"skipped"`
	Flaky        int           `json:"flaky"`
	Attributes   Attributes    `json:"attributes"`
	Mismatches   []Mismatch    `json:"mismatches"`
	Cases        []FailedCase  `json:"cases"`
	CasesOmitted int           `json:"cases_omitted"`
	SuiteOutput  []SuiteOutput `json:"suite_output"`
}

type builder struct {
	o        Options
	r        *Report
	elements int
	seen     map[string]bool
	notPass  map[string]bool
	failures []FailedCase
	errors   []FailedCase
}

func number(s string) *int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return nil
	}
	return &n
}

func total(root *Suite, get func(Suite) string) *int {
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
	for _, l := range strings.Split(text, "\n") {
		l = core.Line(l)
		if strings.TrimSpace(l) == "" || ruleLine.MatchString(l) {
			continue
		}
		out = append(out, l)
	}
	return out
}

func (b *builder) block(texts []string) Block {
	lines, omitted := core.Limit(useful(strings.Join(texts, "\n")), b.o.MaxLines)
	return Block{Lines: lines, LinesOmitted: omitted}
}

func (b *builder) failed(kind, suite string, c Case, res Result) FailedCase {
	f := FailedCase{Kind: kind, Suite: suite, Classname: core.Clean(c.Classname), Name: core.Clean(c.Name), File: core.Clean(c.File), Type: core.Clean(res.Type), Message: core.Clean(res.Message)}
	if n := number(c.Line); n != nil && *n > 0 {
		f.Line = *n
	}
	lines := useful(res.Text)
	if f.Message == "" && len(lines) > 0 {
		f.Message, lines = core.Clean(lines[0]), lines[1:]
	}
	if f.Message == "" {
		f.Message = "no message"
	}
	if len(lines) > 0 && core.Clean(lines[0]) == f.Message {
		lines = lines[1:]
	}
	shown, omitted := core.Limit(lines, b.o.MaxLines)
	f.Text = Block{Lines: shown, LinesOmitted: omitted}
	f.Stdout, f.Stderr = b.block(c.SystemOut), b.block(c.SystemErr)
	return f
}

func (b *builder) walk(s Suite, counted bool) {
	index := b.elements
	b.elements++
	if counted {
		b.r.Suites++
	}
	name := core.Clean(s.Name)
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
			b.failures = append(b.failures, b.failed("failure", name, c, res))
		}
		for _, res := range c.Errors {
			b.errors = append(b.errors, b.failed("error", name, c, res))
		}
	}
	if failing {
		out := SuiteOutput{Suite: name, Stdout: b.block(s.SystemOut), Stderr: b.block(s.SystemErr)}
		if len(out.Stdout.Lines) > 0 || len(out.Stderr.Lines) > 0 {
			b.r.SuiteOutput = append(b.r.SuiteOutput, out)
		}
	}
	for _, child := range s.Suites {
		b.walk(child, true)
	}
}

func Build(root *Suite, o Options) Report {
	r := Report{Tool: "junit", Mismatches: []Mismatch{}, Cases: []FailedCase{}, SuiteOutput: []SuiteOutput{}}
	b := &builder{o: o, r: &r, seen: map[string]bool{}, notPass: map[string]bool{}}
	b.walk(*root, root.XMLName.Local == "testsuite")
	r.Passed = r.Tests - len(b.notPass)
	r.Attributes = Attributes{
		Tests:    total(root, func(s Suite) string { return s.Tests }),
		Failures: total(root, func(s Suite) string { return s.Failures }),
		Errors:   total(root, func(s Suite) string { return s.Errors }),
		Skipped:  total(root, func(s Suite) string { return s.Skipped }),
	}
	for _, m := range []struct {
		name    string
		counted int
		attr    *int
	}{{"tests", r.Tests, r.Attributes.Tests}, {"failures", r.Failures, r.Attributes.Failures}, {"errors", r.Errors, r.Attributes.Errors}, {"skipped", r.Skipped, r.Attributes.Skipped}} {
		if m.attr != nil && *m.attr != m.counted {
			r.Mismatches = append(r.Mismatches, Mismatch{Count: m.name, Counted: m.counted, Attribute: *m.attr})
		}
	}
	all := append(b.failures, b.errors...)
	kept, omitted := core.Limit(all, o.MaxCases)
	r.Cases = append(r.Cases, kept...)
	r.CasesOmitted = omitted
	return r
}

func value(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

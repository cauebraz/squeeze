package jest

import (
	"cmp"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/sanitize"
)

const (
	suiteFailed = "Test suite failed to run"
	separator   = " > "
	bullet      = "● "
)

var (
	stackFrame   = regexp.MustCompile(`^\s*at \S`)
	contextFrame = regexp.MustCompile(`^\s*\d+ \|`)
)

type output struct {
	core.Block
	Hidden int `json:"hidden,omitempty"`
}

type status string

const (
	statusPass status = "pass"
	statusFail status = "fail"
)

type failure struct {
	File   string `json:"file"`
	Test   string `json:"test"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	Output output `json:"output"`
}

type fileError struct {
	File   string `json:"file"`
	Output output `json:"output"`
}

type handle struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Output  output `json:"output"`
}

type testRef struct {
	File   string     `json:"file"`
	Test   string     `json:"test"`
	Status testStatus `json:"status"`
}

type testCounts struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Todo    int `json:"todo"`
}

type fileCounts struct {
	Total       int `json:"total"`
	Passed      int `json:"passed"`
	Failed      int `json:"failed"`
	FailedToRun int `json:"failed_to_run"`
	Skipped     int `json:"skipped"`
}

type snapshotCounts struct {
	Total        int  `json:"total"`
	Failed       int  `json:"failed"`
	Obsolete     int  `json:"obsolete"`
	Written      int  `json:"written"`
	Updated      int  `json:"updated"`
	FilesRemoved int  `json:"files_removed"`
	Failure      bool `json:"failure"`
}

type report struct {
	Tool          string         `json:"tool"`
	ToolVersion   string         `json:"tool_version"`
	Status        status         `json:"status"`
	Root          string         `json:"root"`
	Interrupted   bool           `json:"interrupted"`
	Tests         testCounts     `json:"tests"`
	Files         fileCounts     `json:"files"`
	Snapshots     snapshotCounts `json:"snapshots"`
	RunError      *output        `json:"run_error"`
	FileErrors    []fileError    `json:"file_errors"`
	Failures      []failure      `json:"failures"`
	TestsOmitted  int            `json:"tests_omitted"`
	OpenHandles   []handle       `json:"open_handles"`
	NotRun        []testRef      `json:"not_run"`
	PassingFiles  []string       `json:"passing_files"`
	UnknownStatus map[string]int `json:"unknown_status"`
	Trailing      output         `json:"trailing"`
}

type builder struct {
	o    options
	root string
}

func build(out *rawOutput, o options) report {
	files := slices.Clone(out.TestResults)
	slices.SortStableFunc(files, func(a, b rawTestFile) int { return cmp.Compare(a.Name, b.Name) })
	b := builder{o: o, root: root(out, files)}
	r := report{
		Tool:          "vitest",
		Status:        statusPass,
		Root:          sanitize.Clean(b.root),
		Interrupted:   out.WasInterrupted,
		FileErrors:    []fileError{},
		Failures:      []failure{},
		OpenHandles:   []handle{},
		NotRun:        []testRef{},
		PassingFiles:  []string{},
		UnknownStatus: map[string]int{},
		Trailing:      output{Block: core.HeadBlock(nil, 0)},
		Tests: testCounts{
			Total:   out.NumTotalTests,
			Passed:  out.NumPassedTests,
			Failed:  out.NumFailedTests,
			Skipped: out.NumPendingTests,
			Todo:    out.NumTodoTests,
		},
		Snapshots: snapshotCounts{
			Total:        out.Snapshot.Total,
			Failed:       out.Snapshot.Unmatched,
			Obsolete:     out.Snapshot.Unchecked,
			Written:      out.Snapshot.Added,
			Updated:      out.Snapshot.Updated,
			FilesRemoved: out.Snapshot.FilesRemoved,
			Failure:      out.Snapshot.Failure,
		},
	}
	if out.NumRuntimeErrorTestSuites != nil {
		r.Tool = "jest"
	}
	if !out.Success {
		r.Status = statusFail
	}
	if e := out.RunExecError; e != nil {
		lines := b.lines(splitLines(e.Message + "\n" + stackFrames(e.Stack)))
		r.RunError = &lines
	}
	var failures []failure
	for _, f := range files {
		r.Files.Total++
		name := b.rel(f.Name)
		failed := map[string]bool{}
		fileFailures := 0
		for _, a := range f.AssertionResults {
			test := sanitize.Clean(testName(a))
			switch a.Status {
			case testFailed:
				fileFailures++
				failed[strings.Join(titles(a), " › ")] = true
				fl := failure{File: name, Test: test, Output: b.lines(splitLines(strings.Join(a.FailureMessages, "\n")))}
				if a.Location != nil {
					fl.Line, fl.Column = a.Location.Line, a.Location.Column
				}
				failures = append(failures, fl)
			case testPassed:
			case testPending, testSkipped, testDisabled, testFocused, testTodo:
				r.NotRun = append(r.NotRun, testRef{File: name, Test: test, Status: a.Status})
			default:
				r.UnknownStatus[sanitize.Clean(string(a.Status))]++
			}
		}
		errLines := fileErrorLines(f.Message, failed)
		switch {
		case len(errLines) > 0:
			r.Files.FailedToRun++
			r.FileErrors = append(r.FileErrors, fileError{File: name, Output: b.lines(errLines)})
		case f.Status == testFailed || fileFailures > 0:
			r.Files.Failed++
		case f.Status == testSkipped:
			r.Files.Skipped++
		default:
			r.Files.Passed++
			r.PassingFiles = append(r.PassingFiles, name)
		}
	}
	r.Failures, r.TestsOmitted = core.Limit(failures, o.MaxTests)
	if r.Failures == nil {
		r.Failures = []failure{}
	}
	for _, h := range out.OpenHandles {
		stack := splitLines(h.Stack)
		if len(stack) > 0 && !stackFrame.MatchString(stack[0]) {
			stack = stack[1:]
		}
		r.OpenHandles = append(r.OpenHandles, handle{Name: sanitize.Clean(h.Name), Message: sanitize.Clean(h.Message), Output: b.lines(stack)})
	}
	if out.Trailing != "" {
		r.Trailing = b.lines(splitLines(out.Trailing))
	}
	if r.Status == statusPass && (len(r.FileErrors) > 0 || len(failures) > 0 || r.RunError != nil) {
		r.Status = statusFail
	}
	return r
}

func titles(a rawAssertion) []string {
	return append(slices.Clone(a.AncestorTitles), a.Title)
}

func testName(a rawAssertion) string {
	if a.Title == "" && len(a.AncestorTitles) == 0 {
		return a.FullName
	}
	parts := make([]string, 0, len(a.AncestorTitles)+1)
	for _, t := range titles(a) {
		if t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, separator)
}

func stackFrames(stack string) string {
	lines := splitLines(stack)
	for i, l := range lines {
		if stackFrame.MatchString(l) {
			return strings.Join(lines[i:], "\n")
		}
	}
	return ""
}

func splitLines(s string) []string {
	return slices.Collect(core.SplitLines(strings.TrimRight(s, "\r\n")))
}

func fileErrorLines(message string, failed map[string]bool) []string {
	if strings.TrimSpace(message) == "" {
		return nil
	}
	lines := splitLines(message)
	var out []string
	keep, blocks := true, false
	for _, l := range lines {
		title, ok := strings.CutPrefix(strings.TrimSpace(sanitize.Clean(l)), bullet)
		if ok {
			blocks = true
			title = strings.TrimSpace(title)
			keep = !failed[title]
			if keep && title != suiteFailed {
				out = append(out, title)
			}
			continue
		}
		if keep {
			out = append(out, l)
		}
	}
	if !blocks {
		return lines
	}
	for _, l := range out {
		if strings.TrimSpace(sanitize.Line(l)) != "" {
			return out
		}
	}
	return nil
}

func (b *builder) rel(p string) string {
	p = sanitize.Clean(p)
	if rest, ok := strings.CutPrefix(p, b.root+"/"); ok && b.root != "" {
		return rest
	}
	return p
}

func (b *builder) lines(raw []string) output {
	var out output
	var kept []string
	for _, l := range raw {
		c := sanitize.Line(l)
		if strings.TrimSpace(c) == "" {
			continue
		}
		if !b.o.Full && (internalFrame(c) || contextFrame.MatchString(c)) {
			out.Hidden++
			continue
		}
		kept = append(kept, core.TrimRoot(c, b.root))
	}
	out.Block = core.HeadBlock(core.Dedent(kept), b.o.MaxOutput)
	return out
}

func internalFrame(l string) bool {
	if !stackFrame.MatchString(l) {
		return false
	}
	return strings.Contains(l, "node_modules/") || strings.Contains(l, "(node:") || strings.Contains(l, " node:") || strings.HasSuffix(l, "(<anonymous>)")
}

func root(out *rawOutput, files []rawTestFile) string {
	common := ""
	for i, f := range files {
		dir := path.Dir(sanitize.Clean(f.Name))
		if i == 0 {
			common = dir
			continue
		}
		for common != dir && !strings.HasPrefix(dir, common+"/") && common != "/" && common != "." {
			common = path.Dir(common)
		}
	}
	if common == "" || common == "." || common == "/" || !strings.HasPrefix(common, "/") {
		return ""
	}
	var text strings.Builder
	for _, f := range files {
		text.WriteString(f.Message)
		for _, a := range f.AssertionResults {
			for _, m := range a.FailureMessages {
				text.WriteString(m)
			}
		}
	}
	for _, h := range out.OpenHandles {
		text.WriteString(h.Stack)
	}
	all := text.String()
	for dir := common; dir != "/" && dir != "."; dir = path.Dir(dir) {
		if strings.Contains(all, dir+"/node_modules/") {
			return dir
		}
	}
	return common
}

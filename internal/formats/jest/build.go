package jest

import (
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
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

type Lines struct {
	Lines   []string `json:"lines"`
	Omitted int      `json:"omitted,omitempty"`
	Hidden  int      `json:"hidden,omitempty"`
}

type Failure struct {
	File   string `json:"file"`
	Test   string `json:"test"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	Output Lines  `json:"output"`
}

type FileError struct {
	File   string `json:"file"`
	Output Lines  `json:"output"`
}

type Handle struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Output  Lines  `json:"output"`
}

type TestRef struct {
	File   string `json:"file"`
	Test   string `json:"test"`
	Status string `json:"status"`
}

type Tests struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Todo    int `json:"todo"`
}

type Files struct {
	Total       int `json:"total"`
	Passed      int `json:"passed"`
	Failed      int `json:"failed"`
	FailedToRun int `json:"failed_to_run"`
	Skipped     int `json:"skipped"`
}

type Snapshots struct {
	Total    int  `json:"total"`
	Failed   int  `json:"failed"`
	Obsolete int  `json:"obsolete"`
	Written  int  `json:"written"`
	Updated  int  `json:"updated"`
	Removed  int  `json:"files_removed"`
	Failure  bool `json:"failure"`
}

type Report struct {
	Tool            string         `json:"tool"`
	ToolVersion     string         `json:"tool_version"`
	Status          string         `json:"status"`
	Root            string         `json:"root"`
	Interrupted     bool           `json:"interrupted"`
	Tests           Tests          `json:"tests"`
	Files           Files          `json:"files"`
	Snapshots       Snapshots      `json:"snapshots"`
	RunError        *Lines         `json:"run_error"`
	FileErrors      []FileError    `json:"file_errors"`
	Failures        []Failure      `json:"failures"`
	FailuresOmitted int            `json:"failures_omitted"`
	OpenHandles     []Handle       `json:"open_handles"`
	NotRun          []TestRef      `json:"not_run"`
	Passing         []string       `json:"passing_files"`
	UnknownStatus   map[string]int `json:"unknown_status"`
	Trailing        Lines          `json:"trailing"`
}

type builder struct {
	o    Options
	root string
}

func Build(out *Output, o Options) Report {
	files := append([]TestFile(nil), (*out.TestResults)...)
	sort.SliceStable(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	b := builder{o: o, root: root(out, files)}
	r := Report{
		Tool:          "vitest",
		Status:        "pass",
		Root:          core.Clean(b.root),
		Interrupted:   out.WasInterrupted,
		FileErrors:    []FileError{},
		Failures:      []Failure{},
		OpenHandles:   []Handle{},
		NotRun:        []TestRef{},
		Passing:       []string{},
		UnknownStatus: map[string]int{},
		Trailing:      Lines{Lines: []string{}},
		Tests: Tests{
			Total:   out.NumTotalTests,
			Passed:  out.NumPassedTests,
			Failed:  out.NumFailedTests,
			Skipped: out.NumPendingTests,
			Todo:    out.NumTodoTests,
		},
		Snapshots: Snapshots{
			Total:    out.Snapshot.Total,
			Failed:   out.Snapshot.Unmatched,
			Obsolete: out.Snapshot.Unchecked,
			Written:  out.Snapshot.Added,
			Updated:  out.Snapshot.Updated,
			Removed:  out.Snapshot.FilesRemoved,
			Failure:  out.Snapshot.Failure,
		},
	}
	if out.NumRuntimeErrorTestSuites != nil {
		r.Tool = "jest"
	}
	if !out.Success {
		r.Status = "fail"
	}
	if e := out.RunExecError; e != nil {
		lines := b.lines(splitLines(e.Message + "\n" + stackFrames(e.Stack)))
		r.RunError = &lines
	}
	var failures []Failure
	for _, f := range files {
		r.Files.Total++
		name := b.rel(f.Name)
		failed := map[string]bool{}
		fileFailures := 0
		for _, a := range f.AssertionResults {
			test := core.Clean(testName(a))
			switch a.Status {
			case "failed":
				fileFailures++
				failed[strings.Join(titles(a), " › ")] = true
				fl := Failure{File: name, Test: test, Output: b.lines(splitLines(strings.Join(a.FailureMessages, "\n")))}
				if a.Location != nil {
					fl.Line, fl.Column = a.Location.Line, a.Location.Column
				}
				failures = append(failures, fl)
			case "passed":
			case "pending", "skipped", "disabled", "focused", "todo":
				r.NotRun = append(r.NotRun, TestRef{File: name, Test: test, Status: core.Clean(a.Status)})
			default:
				r.UnknownStatus[core.Clean(a.Status)]++
			}
		}
		fileError := fileErrorLines(f.Message, failed)
		switch {
		case len(fileError) > 0:
			r.Files.FailedToRun++
			r.FileErrors = append(r.FileErrors, FileError{File: name, Output: b.lines(fileError)})
		case f.Status == "failed" || fileFailures > 0:
			r.Files.Failed++
		case f.Status == "skipped":
			r.Files.Skipped++
		default:
			r.Files.Passed++
			r.Passing = append(r.Passing, name)
		}
	}
	r.Failures, r.FailuresOmitted = core.Limit(failures, o.MaxTests)
	if r.Failures == nil {
		r.Failures = []Failure{}
	}
	for _, h := range out.OpenHandles {
		stack := splitLines(h.Stack)
		if len(stack) > 0 && !stackFrame.MatchString(stack[0]) {
			stack = stack[1:]
		}
		r.OpenHandles = append(r.OpenHandles, Handle{Name: core.Clean(h.Name), Message: core.Clean(h.Message), Output: b.lines(stack)})
	}
	if out.Trailing != "" {
		r.Trailing = b.lines(splitLines(out.Trailing))
	}
	if r.Status == "pass" && (len(r.FileErrors) > 0 || len(failures) > 0 || r.RunError != nil) {
		r.Status = "fail"
	}
	return r
}

func titles(a Assertion) []string {
	return append(append([]string(nil), a.AncestorTitles...), a.Title)
}

func testName(a Assertion) string {
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
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

func fileErrorLines(message string, failed map[string]bool) []string {
	if strings.TrimSpace(message) == "" {
		return nil
	}
	lines := splitLines(message)
	var out []string
	keep, blocks := true, false
	for _, l := range lines {
		title, ok := strings.CutPrefix(strings.TrimSpace(core.Clean(l)), bullet)
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
		if strings.TrimSpace(core.Line(l)) != "" {
			return out
		}
	}
	return nil
}

func (b *builder) rel(p string) string {
	p = core.Clean(p)
	if b.root != "" && strings.HasPrefix(p, b.root+"/") {
		return p[len(b.root)+1:]
	}
	return p
}

func (b *builder) lines(raw []string) Lines {
	out := Lines{Lines: []string{}}
	var kept []string
	for _, l := range raw {
		c := core.Line(l)
		if strings.TrimSpace(c) == "" {
			continue
		}
		if !b.o.Full && (internalFrame(c) || contextFrame.MatchString(c)) {
			out.Hidden++
			continue
		}
		if b.root != "" {
			c = strings.ReplaceAll(c, "file://"+b.root+"/", "")
			c = strings.ReplaceAll(c, b.root+"/", "")
		}
		kept = append(kept, c)
	}
	kept = core.Dedent(kept)
	kept, out.Omitted = core.Limit(kept, b.o.MaxOutput)
	out.Lines = append(out.Lines, kept...)
	return out
}

func internalFrame(l string) bool {
	if !stackFrame.MatchString(l) {
		return false
	}
	return strings.Contains(l, "node_modules/") || strings.Contains(l, "(node:") || strings.Contains(l, " node:") || strings.HasSuffix(l, "(<anonymous>)")
}

func root(out *Output, files []TestFile) string {
	common := ""
	for i, f := range files {
		dir := path.Dir(core.Clean(f.Name))
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

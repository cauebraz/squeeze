package gotest

import (
	"regexp"
	"sort"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

var (
	frameLine   = regexp.MustCompile(`^\s*(=== (RUN|PAUSE|CONT|NAME|ATTR|ARTIFACTS)\s|--- (PASS|FAIL|SKIP|BENCH):\s)`)
	summaryLine = regexp.MustCompile(`^(PASS|FAIL|ok  \t.*|FAIL\t.*|\?   \t.*|coverage: .*|exit status \d+|testing: warning: no tests to run)$`)
	benchResult = regexp.MustCompile(`^Benchmark\S*\s+\d+\s+.*/op`)
	spaces      = regexp.MustCompile(`\s+`)
)

type line struct {
	text string
	typ  string
}

type pkgState struct {
	status      string
	elapsed     float64
	failedBuild string
	lines       []line
}

type testState struct {
	pkg, name string
	run       int
	status    string
	elapsed   float64
	lines     []line
	attrs     map[string]string
	artifacts string
}

type builder struct {
	o          Options
	pkgs       map[string]*pkgState
	tests      map[[2]string]*testState
	runs       map[[2]string][]*testState
	builds     map[string][]string
	buildFail  map[string]bool
	text       []string
	outputType bool
	other      map[string]int
	partial    map[[2]string]line
}

func Build(events []Event, o Options) Report {
	b := builder{
		o:         o,
		pkgs:      map[string]*pkgState{},
		tests:     map[[2]string]*testState{},
		runs:      map[[2]string][]*testState{},
		builds:    map[string][]string{},
		buildFail: map[string]bool{},
		other:     map[string]int{},
		partial:   map[[2]string]line{},
	}
	for _, e := range events {
		b.add(e)
	}
	for key, l := range b.partial {
		b.appendLine(key[0], key[1], l)
	}
	return b.finish()
}

func (b *builder) pkg(name string) *pkgState {
	p, ok := b.pkgs[name]
	if !ok {
		p = &pkgState{}
		b.pkgs[name] = p
	}
	return p
}

func (b *builder) test(pkg, name string) *testState {
	key := [2]string{pkg, name}
	if t, ok := b.tests[key]; ok {
		return t
	}
	return b.newRun(pkg, name)
}

func (b *builder) newRun(pkg, name string) *testState {
	key := [2]string{pkg, name}
	t := &testState{pkg: pkg, name: name, run: len(b.runs[key]) + 1, attrs: map[string]string{}}
	b.tests[key] = t
	b.runs[key] = append(b.runs[key], t)
	return t
}

func (b *builder) appendLine(pkg, test string, l line) {
	if test != "" {
		t := b.test(pkg, test)
		t.lines = append(t.lines, l)
		return
	}
	p := b.pkg(pkg)
	p.lines = append(p.lines, l)
}

func splitOutput(s string) []string {
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func (b *builder) add(e Event) {
	if e.Action == "" {
		b.text = append(b.text, e.Text)
		return
	}
	pkg, test := core.Clean(e.Package), core.Clean(e.Test)
	switch e.Action {
	case "build-output":
		ip := core.Clean(e.ImportPath)
		b.builds[ip] = append(b.builds[ip], splitOutput(e.Output)...)
	case "build-fail":
		b.buildFail[core.Clean(e.ImportPath)] = true
	case "start":
		b.pkg(pkg)
	case "run":
		b.pkg(pkg)
		if t, ok := b.tests[[2]string{pkg, test}]; ok && t.status == "" && len(t.lines) == 0 {
			return
		}
		b.newRun(pkg, test)
	case "pause", "cont":
	case "output":
		if e.OutputType != "" {
			b.outputType = true
		}
		key := [2]string{pkg, test}
		pending, ok := b.partial[key]
		text, typ := e.Output, e.OutputType
		if ok {
			text, typ = pending.text+text, pending.typ
			delete(b.partial, key)
		}
		parts := strings.Split(text, "\n")
		for _, l := range parts[:len(parts)-1] {
			b.appendLine(pkg, test, line{text: l, typ: typ})
		}
		if last := parts[len(parts)-1]; last != "" {
			b.partial[key] = line{text: last, typ: typ}
		}
	case "pass", "fail", "skip", "bench":
		elapsed := 0.0
		if e.Elapsed != nil {
			elapsed = *e.Elapsed
		}
		if test != "" {
			t := b.test(pkg, test)
			t.status, t.elapsed = e.Action, elapsed
			return
		}
		p := b.pkg(pkg)
		p.status, p.elapsed = e.Action, elapsed
		if e.FailedBuild != "" {
			p.failedBuild = core.Clean(e.FailedBuild)
		}
	case "attr":
		b.test(pkg, test).attrs[core.Clean(e.Key)] = core.Clean(e.Value)
	case "artifacts":
		b.test(pkg, test).artifacts = core.Clean(e.Path)
	default:
		b.other[core.Clean(e.Action)]++
	}
}

func displayPackage(importPath string) string {
	if i := strings.Index(importPath, " ["); i > 0 {
		return importPath[:i]
	}
	return importPath
}

func (b *builder) finish() Report {
	r := Report{
		Tool:            "go",
		BuildFailures:   []BuildFailure{},
		Failures:        []Failure{},
		PackageFailures: []PackageFailure{},
		Benchmarks:      []Benchmark{},
		Skipped:         []TestRef{},
		Passing:         []PackageRef{},
		NoTests:         []string{},
		Text:            Output{Lines: []string{}},
		Warnings:        []string{},
		Other:           b.other,
	}
	failedPkg := map[string]bool{}
	for _, name := range sortedKeys(b.pkgs) {
		p := b.pkgs[name]
		if p.status == "fail" && p.failedBuild == "" {
			for _, l := range p.lines {
				if strings.TrimSpace(l.text) == "FAIL\t"+name+" [build failed]" {
					p.failedBuild = name
				}
			}
		}
		switch {
		case p.status == "fail" && p.failedBuild != "":
			r.Packages.BuildFailed++
			if _, ok := b.builds[p.failedBuild]; !ok {
				b.buildFail[p.failedBuild] = true
			}
		case p.status == "fail" || p.status == "":
			r.Packages.Failed++
			failedPkg[name] = true
		case p.status == "skip":
			r.Packages.NoTests++
			r.NoTests = append(r.NoTests, name)
		default:
			r.Packages.OK++
			r.Passing = append(r.Passing, PackageRef{Package: name, Elapsed: p.elapsed})
		}
		b.scan(&r, p.lines)
		for _, l := range p.lines {
			text := strings.TrimSpace(l.text)
			switch {
			case benchResult.MatchString(text):
				r.Benchmarks = append(r.Benchmarks, Benchmark{Package: name, Result: spaces.ReplaceAllString(text, " ")})
			case strings.HasPrefix(text, "testing: warning: "):
				r.Warnings = append(r.Warnings, name+": "+core.Clean(text))
			}
		}
	}
	for _, ip := range sortedKeys(b.buildFail) {
		var out []string
		for _, l := range b.builds[ip] {
			if !strings.HasPrefix(l, "# ") {
				out = append(out, l)
			}
		}
		r.BuildFailures = append(r.BuildFailures, BuildFailure{Package: displayPackage(ip), Output: b.output(toLines(out), "")})
	}
	r.Failures = b.failures(&r, failedPkg)
	listed := map[string]bool{}
	for _, f := range r.Failures {
		listed[f.Package] = true
	}
	r.Tests.Failed = len(r.Failures)
	r.Failures, r.FailuresOmitted = core.Limit(r.Failures, b.o.MaxTests)
	for _, name := range sortedKeys(failedPkg) {
		p := b.pkgs[name]
		var kept []line
		for _, l := range p.lines {
			if !b.isFrame(l) && !summaryLine.MatchString(strings.TrimSpace(l.text)) {
				kept = append(kept, l)
			}
		}
		if len(kept) == 0 && listed[name] && p.status != "" {
			continue
		}
		r.PackageFailures = append(r.PackageFailures, PackageFailure{Package: name, Unfinished: p.status == "", Output: b.output(kept, name)})
	}
	b.scan(&r, toLines(b.text))
	r.Text = b.output(toLines(b.text), "")
	r.Status = "pass"
	if r.Packages.Failed+r.Packages.BuildFailed+len(r.BuildFailures)+r.Tests.Failed > 0 {
		r.Status = "fail"
	}
	return r
}

func (b *builder) failures(r *Report, failedPkg map[string]bool) []Failure {
	keys := make([][2]string, 0, len(b.runs))
	for k := range b.runs {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	failing := map[[2]string]bool{}
	for _, k := range keys {
		for _, t := range b.runs[k] {
			b.scan(r, t.lines)
			for _, l := range t.lines {
				if text := strings.TrimSpace(l.text); benchResult.MatchString(text) {
					r.Benchmarks = append(r.Benchmarks, Benchmark{Package: t.pkg, Result: spaces.ReplaceAllString(text, " ")})
				}
			}
			switch {
			case t.status == "fail" || (t.status == "" && failedPkg[t.pkg] && !strings.HasPrefix(t.name, "Benchmark")):
				failing[k] = true
			case t.status == "skip":
				r.Tests.Skipped++
				r.Skipped = append(r.Skipped, TestRef{Package: t.pkg, Test: t.name})
			case t.status == "pass" && !strings.HasPrefix(t.name, "Benchmark"):
				r.Tests.Passed++
			}
		}
	}
	var out []Failure
	for _, k := range keys {
		if !failing[k] {
			continue
		}
		runs := b.runs[k]
		for _, t := range runs {
			if t.status != "fail" && !(t.status == "" && failedPkg[t.pkg] && !strings.HasPrefix(t.name, "Benchmark")) {
				continue
			}
			output := b.testOutput(t)
			if len(output.Lines) == 0 && hasFailingChild(failing, k) {
				continue
			}
			f := Failure{Package: t.pkg, Test: t.name, Elapsed: t.elapsed, Unfinished: t.status == "", Artifacts: t.artifacts, Output: output}
			if len(runs) > 1 {
				f.Run, f.Runs = t.run, len(runs)
			}
			if len(t.attrs) > 0 {
				f.Attrs = t.attrs
			}
			out = append(out, f)
		}
	}
	if out == nil {
		out = []Failure{}
	}
	return out
}

func hasFailingChild(failing map[[2]string]bool, k [2]string) bool {
	for other := range failing {
		if other[0] == k[0] && strings.HasPrefix(other[1], k[1]+"/") {
			return true
		}
	}
	return false
}

func (b *builder) isFrame(l line) bool {
	if b.outputType {
		return l.typ == "frame"
	}
	return frameLine.MatchString(l.text)
}

func (b *builder) testOutput(t *testState) Output {
	var all []line
	errorLines := false
	for _, l := range t.lines {
		if b.isFrame(l) {
			continue
		}
		all = append(all, l)
		if l.typ == "error" || l.typ == "error-continue" {
			errorLines = true
		}
	}
	if b.o.Full || !b.outputType || !errorLines {
		return b.output(all, t.pkg)
	}
	var kept []line
	logs := 0
	panicking, racing := false, false
	for _, l := range all {
		text := strings.TrimSpace(l.text)
		if isPanicStart(l.text) {
			panicking = true
		}
		switch {
		case text == "WARNING: DATA RACE":
			racing = true
		case racing && raceSeparator(text):
			racing = false
			continue
		}
		switch {
		case raceSeparator(text):
		case panicking || racing || l.typ == "error" || l.typ == "error-continue":
			kept = append(kept, l)
		case text != "":
			logs++
		}
	}
	out := b.output(kept, t.pkg)
	out.Logs = logs
	return out
}

func raceSeparator(s string) bool {
	return len(s) >= 10 && strings.Trim(s, "=") == ""
}

func isPanicStart(s string) bool {
	return strings.HasPrefix(s, "panic: ") || strings.HasPrefix(s, "fatal error: ")
}

func (b *builder) output(lines []line, pkg string) Output {
	raw := make([]string, 0, len(lines))
	for _, l := range lines {
		raw = append(raw, l.text)
	}
	out := Output{Lines: []string{}}
	if !b.o.Full {
		raw, out.Stack = filterStack(raw, pkg)
	}
	var cleaned []string
	for _, l := range raw {
		if c := core.Line(l); strings.TrimSpace(c) != "" {
			cleaned = append(cleaned, c)
		}
	}
	cleaned = dedent(cleaned)
	cleaned, out.Omitted = core.Limit(cleaned, b.o.MaxOutput)
	out.Lines = append(out.Lines, cleaned...)
	return out
}

func (b *builder) scan(r *Report, lines []line) {
	for _, l := range lines {
		text := strings.TrimSpace(l.text)
		switch {
		case strings.HasPrefix(text, "panic: test timed out"):
			r.Timeouts++
		case strings.HasPrefix(text, "panic: "):
			r.Panics++
		case strings.HasPrefix(text, "WARNING: DATA RACE"):
			r.Races++
		}
	}
}

func dedent(lines []string) []string {
	indent := -1
	for _, l := range lines {
		n := len(l) - len(strings.TrimLeft(l, " "))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	if indent <= 0 {
		return lines
	}
	for i, l := range lines {
		lines[i] = l[indent:]
	}
	return lines
}

func toLines(texts []string) []line {
	out := make([]line, 0, len(texts))
	for _, t := range texts {
		out = append(out, line{text: t})
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

package gotest

import (
	"cmp"
	"maps"
	"regexp"
	"slices"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/sanitize"
)

var (
	frameLine   = regexp.MustCompile(`^\s*(=== (RUN|PAUSE|CONT|NAME|ATTR|ARTIFACTS)\s|--- (PASS|FAIL|SKIP|BENCH):\s)`)
	summaryLine = regexp.MustCompile(`^(PASS|FAIL|ok  \t.*|FAIL\t.*|\?   \t.*|coverage: .*|exit status \d+|testing: warning: no tests to run)$`)
	benchResult = regexp.MustCompile(`^Benchmark\S*\s+\d+\s+.*/op`)
	spaces      = regexp.MustCompile(`\s+`)
	coverLine   = regexp.MustCompile(`^(?:ok  \t.*\t)?coverage: (.+)$`)
	shuffleLine = regexp.MustCompile(`^-test\.shuffle (-?[0-9]+)$`)
)

const maxCoverage = 20

type line struct {
	text string
	typ  string
}

type pkgState struct {
	status      action
	elapsed     float64
	failedBuild string
	lines       []line
}

type testKey struct {
	pkg, name string
}

type testState struct {
	pkg, name string
	run       int
	status    action
	elapsed   float64
	lines     []line
	attrs     map[string]string
	artifacts string
}

type builder struct {
	o          options
	pkgs       map[string]*pkgState
	tests      map[testKey]*testState
	runs       map[testKey][]*testState
	builds     map[string][]string
	buildFail  map[string]bool
	text       []string
	outputType bool
	other      map[string]int
	partial    map[testKey]line
	seeds      map[string]string
}

func build(events []rawEvent, o options, plain bool) report {
	b := builder{
		o:         o,
		pkgs:      map[string]*pkgState{},
		tests:     map[testKey]*testState{},
		runs:      map[testKey][]*testState{},
		builds:    map[string][]string{},
		buildFail: map[string]bool{},
		other:     map[string]int{},
		partial:   map[testKey]line{},
		seeds:     map[string]string{},
	}
	for _, e := range events {
		b.add(e)
	}
	for _, key := range slices.SortedFunc(maps.Keys(b.partial), compareKeys) {
		b.appendLine(key.pkg, key.name, b.partial[key])
	}
	r := b.finish()
	r.testsUnlisted = plain && !slices.ContainsFunc(events, func(e rawEvent) bool { return e.Action == actionRun })
	return r
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
	key := testKey{pkg, name}
	if t, ok := b.tests[key]; ok {
		return t
	}
	return b.newRun(pkg, name)
}

func (b *builder) newRun(pkg, name string) *testState {
	key := testKey{pkg, name}
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
	if m := shuffleLine.FindStringSubmatch(strings.TrimSpace(sanitize.Line(l.text))); m != nil {
		b.seeds[pkg] = m[1]
		return
	}
	p.lines = append(p.lines, l)
}

func (b *builder) add(e rawEvent) {
	if e.Action == "" {
		b.text = append(b.text, e.Text)
		return
	}
	pkg, test := sanitize.Clean(e.Package), sanitize.Clean(e.Test)
	switch e.Action {
	case actionBuildOutput:
		ip := sanitize.Clean(e.ImportPath)
		b.builds[ip] = append(b.builds[ip], splitOutput(e.Output)...)
	case actionBuildFail:
		b.buildFail[sanitize.Clean(e.ImportPath)] = true
	case actionStart:
		b.pkg(pkg)
	case actionRun:
		b.pkg(pkg)
		if t, ok := b.tests[testKey{pkg, test}]; ok && t.status == "" && len(t.lines) == 0 {
			return
		}
		b.newRun(pkg, test)
	case actionPause, actionCont:
	case actionOutput:
		if e.OutputType != "" {
			b.outputType = true
		}
		key := testKey{pkg, test}
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
	case actionPass, actionFail, actionSkip, actionBench:
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
		if p.status != actionFail {
			p.status, p.elapsed = e.Action, elapsed
		}
		if e.FailedBuild != "" {
			p.failedBuild = sanitize.Clean(e.FailedBuild)
		}
	case actionAttr:
		b.test(pkg, test).attrs[sanitize.Clean(e.Key)] = sanitize.Clean(e.Value)
	case actionArtifacts:
		b.test(pkg, test).artifacts = sanitize.Clean(e.Path)
	default:
		b.other[sanitize.Clean(string(e.Action))]++
	}
}

func coverage(lines []line) string {
	found := ""
	for _, l := range lines {
		if m := coverLine.FindStringSubmatch(strings.TrimSpace(l.text)); m != nil && (found == "" || !strings.HasPrefix(l.text, "ok")) {
			found = sanitize.Clean(m[1])
		}
	}
	return found
}

func displayPackage(importPath string) string {
	if i := strings.Index(importPath, " ["); i > 0 {
		return importPath[:i]
	}
	return importPath
}

func (b *builder) finish() report {
	r := report{
		Tool:                 "go",
		BuildFailures:        []buildFailure{},
		Failures:             []failure{},
		PackageFailures:      []packageFailure{},
		Benchmarks:           []benchmark{},
		Skipped:              []testRef{},
		PassingPackages:      []packageRef{},
		PackagesWithoutTests: []string{},
		Coverage:             []packageCoverage{},
		Shuffle:              []packageSeed{},
		Text:                 output{Block: core.HeadBlock(nil, 0)},
		WarningMessages:      []string{},
		Other:                b.other,
	}
	failedPkg := b.packages(&r)
	b.buildFailures(&r)
	keys := slices.SortedFunc(maps.Keys(b.runs), compareKeys)
	failing := b.countTests(&r, keys, failedPkg)
	r.Failures = b.failures(keys, failing, failedPkg)
	listed := map[string]bool{}
	for _, f := range r.Failures {
		listed[f.Package] = true
	}
	r.Tests.Failed = len(r.Failures)
	r.Failures, r.TestsOmitted = core.Limit(r.Failures, b.o.MaxTests)
	if !b.o.All {
		r.Coverage, r.CoverageOmitted = core.Limit(r.Coverage, maxCoverage)
	}
	b.packageFailures(&r, failedPkg, listed)
	for _, name := range sortedKeys(b.seeds) {
		r.Shuffle = append(r.Shuffle, packageSeed{Package: name, Seed: b.seeds[name], Failed: failedPkg[name]})
	}
	b.scan(&r, toLines(b.text))
	r.Text = b.output(toLines(b.text), "")
	r.Status = statusPass
	if r.Packages.Failed+r.Packages.BuildFailed+len(r.BuildFailures)+r.Tests.Failed > 0 {
		r.Status = statusFail
	}
	return r
}

func (b *builder) packages(r *report) map[string]bool {
	failedPkg := map[string]bool{}
	for _, name := range sortedKeys(b.pkgs) {
		p := b.pkgs[name]
		if p.status == actionFail && p.failedBuild == "" {
			for _, l := range p.lines {
				if strings.TrimSpace(l.text) == "FAIL\t"+name+" [build failed]" {
					p.failedBuild = name
				}
			}
		}
		switch {
		case p.status == actionFail && p.failedBuild != "":
			r.Packages.BuildFailed++
			if _, ok := b.builds[p.failedBuild]; !ok {
				b.buildFail[p.failedBuild] = true
			}
		case p.status == actionFail || p.status == "":
			r.Packages.Failed++
			failedPkg[name] = true
		case p.status == actionSkip:
			r.Packages.NoTests++
			r.PackagesWithoutTests = append(r.PackagesWithoutTests, name)
		default:
			r.Packages.OK++
			r.PassingPackages = append(r.PassingPackages, packageRef{Package: name, Elapsed: p.elapsed})
		}
		b.scan(r, p.lines)
		if c := coverage(p.lines); c != "" {
			r.Coverage = append(r.Coverage, packageCoverage{Package: name, Coverage: c})
		}
		for _, l := range p.lines {
			text := strings.TrimSpace(l.text)
			switch {
			case benchResult.MatchString(text):
				r.Benchmarks = append(r.Benchmarks, benchmark{Package: name, Result: spaces.ReplaceAllString(text, " ")})
			case strings.HasPrefix(text, "testing: warning: "):
				r.WarningMessages = append(r.WarningMessages, name+": "+sanitize.Clean(text))
			}
		}
	}
	return failedPkg
}

func (b *builder) buildFailures(r *report) {
	for _, ip := range sortedKeys(b.buildFail) {
		var out []string
		for _, l := range b.builds[ip] {
			if !strings.HasPrefix(l, "# ") {
				out = append(out, l)
			}
		}
		r.BuildFailures = append(r.BuildFailures, buildFailure{Package: displayPackage(ip), Output: b.output(toLines(out), "")})
	}
}

func (b *builder) packageFailures(r *report, failedPkg, listed map[string]bool) {
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
		r.PackageFailures = append(r.PackageFailures, packageFailure{Package: name, Unfinished: p.status == "", Output: b.output(kept, name)})
	}
}

func failed(t *testState, failedPkg map[string]bool) bool {
	return t.status == actionFail || (t.status == "" && failedPkg[t.pkg] && !strings.HasPrefix(t.name, "Benchmark"))
}

func (b *builder) countTests(r *report, keys []testKey, failedPkg map[string]bool) map[testKey]bool {
	failing := map[testKey]bool{}
	for _, k := range keys {
		for _, t := range b.runs[k] {
			b.scan(r, t.lines)
			for _, l := range t.lines {
				if text := strings.TrimSpace(l.text); benchResult.MatchString(text) {
					r.Benchmarks = append(r.Benchmarks, benchmark{Package: t.pkg, Result: spaces.ReplaceAllString(text, " ")})
				}
			}
			switch {
			case failed(t, failedPkg):
				failing[k] = true
			case t.status == actionSkip:
				r.Tests.Skipped++
				r.Skipped = append(r.Skipped, testRef{Package: t.pkg, Test: t.name})
			case t.status == actionPass && !strings.HasPrefix(t.name, "Benchmark"):
				r.Tests.Passed++
			}
		}
	}
	return failing
}

func (b *builder) failures(keys []testKey, failing map[testKey]bool, failedPkg map[string]bool) []failure {
	parents := failingParents(failing)
	out := []failure{}
	for _, k := range keys {
		if !failing[k] {
			continue
		}
		runs := b.runs[k]
		for _, t := range runs {
			if !failed(t, failedPkg) {
				continue
			}
			lines := b.testOutput(t)
			if len(lines.Lines) == 0 && parents[k] {
				continue
			}
			f := failure{Package: t.pkg, Test: t.name, Elapsed: t.elapsed, Unfinished: t.status == "", Artifacts: t.artifacts, Output: lines}
			if len(runs) > 1 {
				f.Run, f.Runs = t.run, len(runs)
			}
			if len(t.attrs) > 0 {
				f.Attrs = t.attrs
			}
			out = append(out, f)
		}
	}
	return out
}

func failingParents(failing map[testKey]bool) map[testKey]bool {
	parents := map[testKey]bool{}
	for k := range failing {
		for i := range len(k.name) {
			if k.name[i] == '/' {
				parents[testKey{k.pkg, k.name[:i]}] = true
			}
		}
	}
	return parents
}

func (b *builder) scan(r *report, lines []line) {
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

func compareKeys(a, b testKey) int {
	return cmp.Or(cmp.Compare(a.pkg, b.pkg), cmp.Compare(a.name, b.name))
}

func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

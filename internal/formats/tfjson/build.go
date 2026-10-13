package tfjson

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

var actionRanks = map[string]int{
	"delete": 0, "replace": 1, "remove": 2, "update": 3, "create": 4,
	"read": 5, "import": 6, "move": 7, "noop": 8,
}

var ephemeralActions = map[string]bool{"open": true, "renew": true, "close": true}

var failureTypes = map[string]bool{
	"apply_errored": true, "provision_errored": true, "action_errored": true, "ephemeral_op_errored": true,
}

var failingTestStatus = map[string]bool{"fail": true, "error": true}

func rank(action string) int {
	if r, ok := actionRanks[action]; ok {
		return r
	}
	return len(actionRanks)
}

type builder struct {
	o        Options
	r        Report
	changes  map[string]int
	runs     map[[2]string]int
	files    map[string]int
	diags    []tfdiag.Diagnostic
	timings  []Timing
	sawTest  bool
	sawInit  bool
	sawPlan  bool
	sawApply bool
}

func Build(lines []Line, o Options) Report {
	b := builder{
		o:       o,
		changes: map[string]int{},
		runs:    map[[2]string]int{},
		files:   map[string]int{},
		r: Report{
			Tool:     "terraform",
			Changes:  []Change{},
			Drift:    []Change{},
			Deferred: []Deferred{},
			Failures: []Failure{},
			Policies: []PolicyResult{},
			Outputs:  []Output{},
			Other:    map[string]int{},
			Applied:  Applied{Slowest: []Timing{}},
		},
	}
	for _, l := range lines {
		b.add(l)
	}
	return b.finish()
}

func (b *builder) other(key, message string) {
	b.r.Other[key]++
	if message != "" {
		b.r.otherMessages = append(b.r.otherMessages, message)
	}
}

func (b *builder) add(l Line) {
	if l.Type == "" {
		b.other("text", clean(l.Text))
		return
	}
	if strings.HasPrefix(l.Type, "test_") {
		b.sawTest = true
	}
	if l.Type == "init_output" || strings.HasPrefix(l.Type, "provider_") || strings.HasPrefix(l.Type, "migration_") {
		b.sawInit = true
	}
	if l.Malformed || (l.Type != "diagnostic" && (l.Level == "error" || l.Level == "warn")) {
		b.fallback(l)
		return
	}
	switch {
	case l.Type == "version":
		b.r.ToolVersion = clean(l.Terraform)
	case l.Type == "diagnostic" && l.Diagnostic != nil:
		b.diagnostic(tfdiag.Convert(*l.Diagnostic), l.AtTestFile, l.AtTestRun)
	case l.Type == "planned_change" && l.Change != nil:
		b.sawPlan = true
		b.change(*l.Change)
	case l.Type == "resource_drift" && l.Change != nil:
		b.r.Drift = append(b.r.Drift, convertChange(*l.Change))
	case l.Type == "deferred_change" && l.Deferred != nil && l.Deferred.Change != nil:
		c := convertChange(*l.Deferred.Change)
		if c.Action != "noop" {
			b.r.Deferred = append(b.r.Deferred, Deferred{Address: c.Address, Action: c.Action, Reason: clean(l.Deferred.Reason)})
		}
	case l.Type == "change_summary" && l.Changes != nil:
		c := l.Changes
		b.r.Summary = &Summary{
			Operation: c.Operation, Add: c.Add, Change: c.Change, Remove: c.Remove,
			Import: c.Import, ActionInvocation: c.ActionInvocation, ActionFail: c.ActionFail,
		}
	case l.Type == "apply_complete" && l.Hook != nil:
		b.applied(*l.Hook, l.Message)
	case failureTypes[l.Type]:
		if l.Type == "apply_errored" {
			b.sawApply = true
		}
		f := Failure{Type: l.Type, Message: clean(l.Message)}
		if l.Hook != nil {
			f.Address = clean(l.Hook.Resource.Addr)
		}
		b.r.Failures = append(b.r.Failures, f)
	case l.Type == "outputs" && l.Outputs != nil:
		b.outputs(l.Outputs)
	case l.Type == "test_summary" && l.TestSummary != nil:
		t := b.tests()
		s := l.TestSummary
		t.Status, t.Passed, t.Failed, t.Errored, t.Skipped = clean(s.Status), s.Passed, s.Failed, s.Errored, s.Skipped
	case l.Type == "test_run" && l.TestRun != nil:
		b.testRun(*l.TestRun)
	case l.Type == "test_file" && l.TestFile != nil:
		b.testFile(clean(l.TestFile.Path), clean(l.TestFile.Status))
	case l.Type == "policy_result" && l.Result != "" && l.Result != "AllowResult":
		p := PolicyResult{Policy: clean(l.Policy), Target: clean(l.Target), Result: clean(l.Result)}
		if l.PolicyMeta != nil {
			p.Enforcement = clean(l.PolicyMeta.EnforcementLevel)
		}
		b.r.Policies = append(b.r.Policies, p)
	case l.Type == "provider_version_installation_complete":
		b.r.Providers++
	default:
		b.fallback(l)
	}
}

func (b *builder) fallback(l Line) {
	switch l.Level {
	case "error":
		b.diagnostic(tfdiag.Diagnostic{Severity: "error", Summary: stripPrefix(l.Message, "Error: ")}, l.AtTestFile, l.AtTestRun)
	case "warn":
		b.diagnostic(tfdiag.Diagnostic{Severity: "warning", Summary: stripPrefix(l.Message, "Warning: ")}, l.AtTestFile, l.AtTestRun)
	default:
		b.other(clean(l.Type), clean(l.Message))
	}
}

func convertChange(c streamChange) Change {
	out := Change{Address: clean(c.Resource.Addr), Action: clean(c.Action), Reason: clean(c.Reason)}
	if c.PreviousResource != nil && c.PreviousResource.Addr != c.Resource.Addr {
		out.PreviousAddress = clean(c.PreviousResource.Addr)
	}
	if c.Importing != nil {
		out.ImportID = clean(c.Importing.ID)
	}
	return out
}

func (b *builder) change(sc streamChange) {
	if ephemeralActions[sc.Action] {
		b.other("ephemeral", "")
		return
	}
	c := convertChange(sc)
	if i, ok := b.changes[c.Address]; ok {
		b.r.Changes[i] = c
		return
	}
	b.changes[c.Address] = len(b.r.Changes)
	b.r.Changes = append(b.r.Changes, c)
}

func (b *builder) applied(h streamHook, message string) {
	var action string
	_ = json.Unmarshal(h.Action, &action)
	if action == "read" {
		b.other("refresh", clean(message))
		return
	}
	b.sawApply = true
	b.r.Applied.Count++
	b.timings = append(b.timings, Timing{Address: clean(h.Resource.Addr), Seconds: h.Elapsed})
}

func (b *builder) outputs(outputs map[string]streamOutput) {
	names := make([]string, 0, len(outputs))
	for name := range outputs {
		names = append(names, name)
	}
	sort.Strings(names)
	b.r.Outputs = make([]Output, 0, len(names))
	for _, name := range names {
		o := outputs[name]
		out := Output{Name: clean(name), Action: clean(o.Action), Sensitive: o.Sensitive}
		if !o.Sensitive && isScalar(o.Value) {
			out.Value = o.Value
		}
		b.r.Outputs = append(b.r.Outputs, out)
	}
}

func isScalar(v json.RawMessage) bool {
	v = bytes.TrimSpace(v)
	return len(v) > 0 && v[0] != '{' && v[0] != '[' && json.Valid(v)
}

func (b *builder) tests() *Tests {
	if b.r.Tests == nil {
		b.r.Tests = &Tests{Runs: []TestRun{}, Files: []TestFile{}}
	}
	return b.r.Tests
}

func (b *builder) testRun(s streamTestRun) {
	s.Path, s.Run, s.Status = clean(s.Path), clean(s.Run), clean(s.Status)
	t := b.tests()
	key := [2]string{s.Path, s.Run}
	i, ok := b.runs[key]
	if !ok {
		i = len(t.Runs)
		b.runs[key] = i
		t.Runs = append(t.Runs, TestRun{File: s.Path, Run: s.Run, Diagnostics: []tfdiag.Diagnostic{}})
	}
	if s.Status != "" {
		t.Runs[i].Status = s.Status
	}
	if s.Elapsed != nil {
		t.Runs[i].ElapsedMs = s.Elapsed
	}
}

func (b *builder) testFile(path, status string) *TestFile {
	t := b.tests()
	i, ok := b.files[path]
	if !ok {
		i = len(t.Files)
		b.files[path] = i
		t.Files = append(t.Files, TestFile{File: path, Diagnostics: []tfdiag.Diagnostic{}})
	}
	if status != "" {
		t.Files[i].Status = status
	}
	return &t.Files[i]
}

func (b *builder) diagnostic(d tfdiag.Diagnostic, file, run string) {
	file, run = clean(file), clean(run)
	switch {
	case file != "" && run != "":
		b.testRun(streamTestRun{Path: file, Run: run})
		i := b.runs[[2]string{file, run}]
		b.r.Tests.Runs[i].Diagnostics = append(b.r.Tests.Runs[i].Diagnostics, d)
	case file != "":
		f := b.testFile(file, "")
		f.Diagnostics = append(f.Diagnostics, d)
	default:
		b.diags = append(b.diags, d)
	}
	switch d.Severity {
	case "error":
		b.r.Errors++
	case "warning":
		b.r.Warnings++
	}
}

func (b *builder) finish() Report {
	r := b.r
	r.Operation = b.operation()
	sort.SliceStable(r.Changes, func(i, j int) bool { return lessChange(r.Changes[i], r.Changes[j]) })
	sort.SliceStable(r.Drift, func(i, j int) bool { return lessChange(r.Drift[i], r.Drift[j]) })
	sort.SliceStable(r.Deferred, func(i, j int) bool {
		a, c := r.Deferred[i], r.Deferred[j]
		if rank(a.Action) != rank(c.Action) {
			return rank(a.Action) < rank(c.Action)
		}
		return a.Address < c.Address
	})
	if !b.o.All {
		visible := r.Changes[:0]
		for _, c := range r.Changes {
			if c.Action != "noop" {
				visible = append(visible, c)
			}
		}
		r.Changes = visible
	}
	if b.o.MaxResources > 0 && len(r.Changes) > b.o.MaxResources {
		r.Omitted = len(r.Changes) - b.o.MaxResources
		r.Changes = r.Changes[:b.o.MaxResources]
	}
	sort.SliceStable(b.timings, func(i, j int) bool { return b.timings[i].Seconds > b.timings[j].Seconds })
	for _, t := range b.timings {
		if len(r.Applied.Slowest) == 3 || t.Seconds <= 0 {
			break
		}
		r.Applied.Slowest = append(r.Applied.Slowest, t)
	}
	r.Diagnostics, r.DiagnosticsOmitted = tfdiag.Limit(tfdiag.Normalize(b.diags), b.o.MaxDiagnostics)
	if r.Diagnostics == nil {
		r.Diagnostics = []tfdiag.Diagnostic{}
	}
	if r.Tests != nil {
		for i := range r.Tests.Runs {
			r.Tests.Runs[i].Diagnostics = tfdiag.Normalize(r.Tests.Runs[i].Diagnostics)
		}
		for i := range r.Tests.Files {
			r.Tests.Files[i].Diagnostics = tfdiag.Normalize(r.Tests.Files[i].Diagnostics)
		}
	}
	return r
}

func lessChange(a, b Change) bool {
	if rank(a.Action) != rank(b.Action) {
		return rank(a.Action) < rank(b.Action)
	}
	return a.Address < b.Address
}

func (b *builder) operation() string {
	switch {
	case b.r.Summary != nil && b.r.Summary.Operation == "plan" && b.sawApply:
		return "apply"
	case b.r.Summary != nil && b.r.Summary.Operation != "":
		return b.r.Summary.Operation
	case b.sawTest:
		return "test"
	case b.sawPlan:
		return "plan"
	case b.sawInit:
		return "init"
	default:
		return ""
	}
}

func stripPrefix(message, prefix string) string {
	return clean(strings.TrimPrefix(clean(message), prefix))
}

func clean(s string) string {
	return core.Clean(s)
}

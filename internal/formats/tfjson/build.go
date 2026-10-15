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

var policyOutcomes = map[string]string{
	"AllowResult": "pass", "pass": "pass", "n/a": "pass",
	"DenyResult": "fail", "fail": "fail",
	"UnknownResult": "unknown", "unknown": "unknown",
	"InvalidResult": "error", "PolicyErrorResult": "error", "SetupErrorResult": "error", "error": "error",
}

func policyOutcome(result string) string {
	if o, ok := policyOutcomes[result]; ok {
		return o
	}
	return "error"
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
	changes  map[Change]bool
	actions  map[ActionInvocation]bool
	lists    map[string]int
	found    map[string][]FoundResource
	runs     map[[2]string]int
	files    map[string]int
	diags    []tfdiag.Diagnostic
	timings  []Timing
	sawTest  bool
	sawQuery bool
	sawInit  bool
	sawPlan  bool
	sawApply bool
}

func Build(lines []Line, o Options) Report {
	b := builder{
		o:       o,
		changes: map[Change]bool{},
		actions: map[ActionInvocation]bool{},
		lists:   map[string]int{},
		found:   map[string][]FoundResource{},
		runs:    map[[2]string]int{},
		files:   map[string]int{},
		r: Report{
			Tool:     "terraform",
			Changes:  []Change{},
			Drift:    []Change{},
			Deferred: []Deferred{},
			Actions:  []ActionInvocation{},
			Lists:    []ListBlock{},
			Found:    []FoundResource{},
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
		b.other("text", core.Clean(l.Text))
		return
	}
	if strings.HasPrefix(l.Type, "test_") {
		b.sawTest = true
	}
	if l.Type == "init_output" || strings.HasPrefix(l.Type, "provider_") || strings.HasPrefix(l.Type, "migration_") {
		b.sawInit = true
	}
	left := b.leftInState(l)
	if l.Malformed || (!isDiagnostic(l) && (l.Level == "error" || l.Level == "warn")) {
		b.fallback(l)
		return
	}
	if left {
		return
	}
	switch {
	case l.Type == "version" && l.Tofu != "":
		b.r.Tool = "tofu"
		b.r.ToolVersion = core.Clean(l.Tofu)
	case l.Type == "version":
		b.r.ToolVersion = core.Clean(l.Terraform)
	case l.Type == "diagnostic" && l.Diagnostic != nil:
		b.diagnostic(tfdiag.Convert(*l.Diagnostic), l.AtTestFile, l.AtTestRun)
	case l.Type == "policy_diagnostic" && l.PolicyDiag != nil:
		b.diagnostic(policyDiagnostic(*l.PolicyDiag, l.Target), l.AtTestFile, l.AtTestRun)
	case l.Type == "planned_change" && l.Change != nil:
		b.sawPlan = true
		b.change(*l.Change)
	case l.Type == "planned_action_invocation" && l.Invocation != nil:
		b.sawPlan = true
		b.invocation(*l.Invocation)
	case l.Type == "action_complete":
		b.r.ActionsCompleted++
	case l.Type == "list_start" && l.ListStart != nil:
		b.list(*l.ListStart)
	case l.Type == "list_resource_found" && l.ListFound != nil:
		f := l.ListFound
		b.list(streamList{Address: f.Address})
		addr := core.Clean(f.Address)
		b.found[addr] = append(b.found[addr], FoundResource{Address: addr, DisplayName: core.Clean(f.DisplayName), Identity: f.Identity})
	case l.Type == "list_complete" && l.ListDone != nil:
		lb := b.list(*l.ListDone)
		lb.Total, lb.Complete = l.ListDone.Total, true
	case l.Type == "resource_drift" && l.Change != nil:
		b.r.Drift = append(b.r.Drift, convertChange(*l.Change))
	case l.Type == "deferred_change" && l.Deferred != nil && l.Deferred.Change != nil:
		c := convertChange(*l.Deferred.Change)
		if c.Action != "noop" {
			b.r.Deferred = append(b.r.Deferred, Deferred{Address: c.Address, Action: c.Action, Reason: core.Clean(l.Deferred.Reason)})
		}
	case l.Type == "change_summary" && l.Changes != nil:
		c := l.Changes
		b.r.Summary = &Summary{
			Operation: core.Clean(c.Operation), Add: c.Add, Change: c.Change, Remove: c.Remove,
			Import: c.Import, ActionInvocation: c.ActionInvocation, ActionFail: c.ActionFail,
		}
	case l.Type == "apply_complete" && l.Hook != nil:
		b.applied(*l.Hook, l.Message)
	case failureTypes[l.Type]:
		if l.Type == "apply_errored" {
			b.sawApply = true
		}
		f := Failure{Type: l.Type, Message: core.Clean(l.Message)}
		if l.Hook != nil {
			f.Address = core.Clean(hookAddress(*l.Hook))
		}
		b.r.Failures = append(b.r.Failures, f)
	case l.Type == "outputs" && l.Outputs != nil:
		b.outputs(l.Outputs)
	case l.Type == "test_summary" && l.TestSummary != nil:
		t := b.tests()
		s := l.TestSummary
		t.Status, t.Passed, t.Failed, t.Errored, t.Skipped = core.Clean(s.Status), s.Passed, s.Failed, s.Errored, s.Skipped
	case l.Type == "test_run" && l.TestRun != nil:
		b.testRun(*l.TestRun)
	case l.Type == "test_file" && l.TestFile != nil:
		b.testFile(core.Clean(l.TestFile.Path), core.Clean(l.TestFile.Status))
	case l.Type == "policy_result" && l.Result != "":
		var enforcement string
		if l.PolicyMeta != nil {
			enforcement = l.PolicyMeta.EnforcementLevel
		}
		if !b.policy(l.Policy, l.Target, l.Result, enforcement) {
			b.other(l.Type, policyLine(PolicyResult{Policy: core.Clean(l.Policy), Target: core.Clean(l.Target), Result: core.Clean(l.Result)}))
		}
	case l.Type == "policy_info" && l.PolicyInfo != nil:
		b.other(l.Type, policyInfo(l))
	case l.Type == "policy_query_summary" && l.ListBlock != "":
		b.querySummary(l.Query)
	case l.Type == "provider_version_installation_complete":
		b.r.Providers++
	default:
		b.fallback(l)
	}
}

func isDiagnostic(l Line) bool {
	return l.Type == "diagnostic" || (l.Type == "policy_diagnostic" && l.PolicyDiag != nil)
}

func policyDiagnostic(raw tfdiag.Raw, target string) tfdiag.Diagnostic {
	d := tfdiag.Convert(raw)
	if d.Address == "" {
		d.Address = core.Clean(target)
	}
	return d
}

func (b *builder) policy(name, target, result, enforcement string) bool {
	outcome := policyOutcome(result)
	if outcome == "pass" {
		return false
	}
	b.r.Policies = append(b.r.Policies, PolicyResult{
		Policy: core.Clean(name), Target: core.Clean(target), Result: core.Clean(result),
		Outcome: outcome, Enforcement: core.Clean(enforcement),
	})
	return true
}

func policyInfo(l Line) string {
	parts := []string{"policy info:"}
	if l.PolicyMeta != nil && l.PolicyMeta.PolicyName != "" {
		parts = append(parts, core.Clean(l.PolicyMeta.PolicyName))
	}
	if l.Target != "" {
		parts = append(parts, "for "+core.Clean(l.Target))
	}
	return strings.Join(parts, " ") + ": " + core.Clean(l.PolicyInfo.Message)
}

func (b *builder) querySummary(results []streamQueryResult) {
	for _, r := range results {
		listed := false
		for _, p := range r.Policies {
			for _, raw := range p.Diagnostics {
				b.diagnostic(policyDiagnostic(raw, r.Target), "", "")
			}
			if b.policy(p.Meta.PolicyName, r.Target, p.Result, p.Meta.EnforcementLevel) {
				listed = true
			}
		}
		if !listed {
			b.policy("", r.Target, r.Result, "")
		}
	}
}

func (b *builder) fallback(l Line) {
	switch l.Level {
	case "error":
		b.diagnostic(tfdiag.Diagnostic{Severity: "error", Summary: stripPrefix(l.Message, "Error: ")}, l.AtTestFile, l.AtTestRun)
	case "warn":
		b.diagnostic(tfdiag.Diagnostic{Severity: "warning", Summary: stripPrefix(l.Message, "Warning: ")}, l.AtTestFile, l.AtTestRun)
	default:
		b.other(core.Clean(l.Type), core.Clean(l.Message))
	}
}

func hookAddress(h streamHook) string {
	if h.Resource.Addr != "" {
		return h.Resource.Addr
	}
	var action streamActionAddr
	_ = json.Unmarshal(h.Action, &action)
	return action.Addr
}

func convertChange(c streamChange) Change {
	out := Change{Address: core.Clean(c.Resource.Addr), Action: core.Clean(c.Action), Reason: core.Clean(c.Reason)}
	if c.PreviousResource != nil && c.PreviousResource.Addr != c.Resource.Addr {
		out.PreviousAddress = core.Clean(c.PreviousResource.Addr)
	}
	if c.Importing != nil {
		out.ImportID = core.Clean(c.Importing.ID)
	}
	return out
}

func (b *builder) change(sc streamChange) {
	if ephemeralActions[sc.Action] {
		b.other("ephemeral", "")
		return
	}
	c := convertChange(sc)
	if b.changes[c] {
		return
	}
	b.changes[c] = true
	b.r.Changes = append(b.r.Changes, c)
}

func (b *builder) list(s streamList) *ListBlock {
	b.sawQuery = true
	addr := core.Clean(s.Address)
	i, ok := b.lists[addr]
	if !ok {
		i = len(b.r.Lists)
		b.lists[addr] = i
		b.r.Lists = append(b.r.Lists, ListBlock{Address: addr})
	}
	lb := &b.r.Lists[i]
	if s.ResourceType != "" {
		lb.ResourceType = core.Clean(s.ResourceType)
	}
	return lb
}

func (b *builder) invocation(s streamInvocation) {
	a := ActionInvocation{Address: core.Clean(s.Action.Addr)}
	switch {
	case s.Lifecycle != nil:
		a.Event = core.Clean(s.Lifecycle.Event)
		a.Resource = core.Clean(s.Lifecycle.Resource.Addr)
	case s.Invoke != nil:
		a.Invoked = true
	}
	if b.actions[a] {
		return
	}
	b.actions[a] = true
	b.r.Actions = append(b.r.Actions, a)
}

func (b *builder) applied(h streamHook, message string) {
	var action string
	_ = json.Unmarshal(h.Action, &action)
	if action == "read" {
		b.other("refresh", core.Clean(message))
		return
	}
	b.sawApply = true
	b.r.Applied.Count++
	b.timings = append(b.timings, Timing{Address: core.Clean(h.Resource.Addr), Seconds: h.Elapsed})
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
		out := Output{Name: core.Clean(name), Action: core.Clean(o.Action), Sensitive: o.Sensitive}
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
		b.r.Tests = &Tests{Runs: []TestRun{}, Files: []TestFile{}, Left: []LeftInState{}}
	}
	return b.r.Tests
}

func (b *builder) leftInState(l Line) bool {
	switch {
	case l.Type == "test_cleanup" && l.Cleanup != nil:
		b.left(l, "cleanup_failed", "", l.Cleanup.FailedResources)
		b.left(l, "skip_cleanup", "", l.Cleanup.SkippedResources)
	case l.Type == "test_interrupt" && l.Interrupt != nil:
		b.left(l, "interrupted", "", l.Interrupt.State)
		keys := make([]string, 0, len(l.Interrupt.States))
		for k := range l.Interrupt.States {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.left(l, "interrupted", k, l.Interrupt.States[k])
		}
		planned := make([]streamTestResource, 0, len(l.Interrupt.Planned))
		for _, p := range l.Interrupt.Planned {
			planned = append(planned, streamTestResource{Instance: p})
		}
		b.left(l, "planned", "", planned)
	default:
		return false
	}
	return true
}

func (b *builder) left(l Line, reason, state string, resources []streamTestResource) {
	if len(resources) == 0 {
		return
	}
	out := make([]LeftResource, 0, len(resources))
	for _, r := range resources {
		out = append(out, LeftResource{Address: core.Clean(r.Instance), DeposedKey: core.Clean(r.DeposedKey)})
	}
	t := b.tests()
	t.Left = append(t.Left, LeftInState{
		File: core.Clean(l.AtTestFile), Run: core.Clean(l.AtTestRun), Reason: reason, State: core.Clean(state), Resources: out,
	})
}

func (b *builder) testRun(s streamTestRun) {
	s.Path, s.Run, s.Status = core.Clean(s.Path), core.Clean(s.Run), core.Clean(s.Status)
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
	file, run = core.Clean(file), core.Clean(run)
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
			} else {
				r.NoopHidden++
			}
		}
		r.Changes = visible
	}
	r.Changes, r.Omitted = core.Limit(r.Changes, b.o.MaxResources)
	sort.SliceStable(r.Lists, func(i, j int) bool { return r.Lists[i].Address < r.Lists[j].Address })
	for i := range r.Lists {
		found := b.found[r.Lists[i].Address]
		if !r.Lists[i].Complete {
			r.Lists[i].Total = len(found)
		}
		r.Found = append(r.Found, found...)
	}
	r.Found, r.FoundOmitted = core.Limit(r.Found, b.o.MaxResources)
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
	case b.sawQuery:
		return "query"
	case b.sawPlan:
		return "plan"
	case b.sawInit:
		return "init"
	default:
		return ""
	}
}

func stripPrefix(message, prefix string) string {
	return core.Clean(strings.TrimPrefix(core.Clean(message), prefix))
}

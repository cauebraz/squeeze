package tfplan

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/sanitize"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

func conditions(r report) []string {
	s := r.Summary
	checkBlock, otherCheck := false, false
	for _, c := range r.Checks {
		if c.Kind == "check" {
			checkBlock = true
		} else {
			otherCheck = true
		}
	}
	errorCount, warningCount := tfdiag.Count(r.Diagnostics)
	return core.Conditions(map[string]bool{
		"change":     s.changed(),
		"check":      len(r.Checks) > 0,
		"deferred":   s.Deferred+s.DeferredActions > 0,
		"destroy":    s.Destroy > 0,
		"drift":      s.Drift > 0,
		"error":      s.Errored || errorCount > 0,
		"warning":    checkBlock || (otherCheck && !s.Errored) || warningCount > 0,
		"fail":       s.Errored || !s.Complete || errorCount > 0,
		"forget":     s.Forget > 0,
		"import":     s.Import > 0,
		"incomplete": !s.Complete,
		"move":       s.Move > 0,
		"replace":    s.Replace > 0,
	})
}

func build(p *rawPlan, o options) report {
	r := report{
		Tool:        "terraform",
		ToolVersion: sanitize.Clean(p.TerraformVersion),
		Summary: summary{
			Errored:  p.Errored,
			Complete: p.Complete == nil || *p.Complete,
		},
		Changes:         []resourceChange{},
		Drift:           []resourceChange{},
		Deferred:        []deferredChange{},
		DeferredActions: []deferredAction{},
		Actions:         []tfdiag.ActionInvocation{},
		Outputs:         []outputChange{},
	}
	r.Diagnostics = tfdiag.Normalize(tfdiag.FromLines(p.Trailing, ""))
	for _, pr := range p.ResourceChanges {
		c := convert(pr, o, nil)
		count(&r.Summary, c)
		if c.Action == actionNoop && !moved(c) && !o.IncludeNoop {
			r.NoopHidden++
			continue
		}
		r.Changes = append(r.Changes, c)
	}
	sortChanges(r.Changes)
	r.Changes, r.Omitted = core.Limit(r.Changes, o.MaxResources)
	buildDrift(&r, p, o)
	buildDeferred(&r, p, o)
	r.Actions = convertActions(p.ActionInvocations)
	r.Summary.Actions = len(r.Actions)
	for _, d := range p.DeferredActions {
		r.DeferredActions = append(r.DeferredActions, deferredAction{ActionInvocation: convertAction(d.ActionInvocation), Reason: sanitize.Clean(d.Reason)})
	}
	r.Summary.DeferredActions = len(r.DeferredActions)
	outputs := convertOutputs(p.OutputChanges, o.IncludeNoop)
	r.Outputs = outputs
	for _, out := range outputs {
		if out.Action != actionNoop {
			r.Summary.Outputs++
		}
	}
	r.Checks = convertChecks(p.Checks)
	r.Summary.ChecksFailed = len(r.Checks)
	return r
}

func convertChecks(checks []rawCheck) []checkResult {
	out := []checkResult{}
	for _, c := range checks {
		kind := sanitize.Clean(c.Address.Kind)
		object := sanitize.Clean(c.Address.ToDisplay)
		if len(c.Instances) == 0 {
			if checkFailed(c.Status) {
				out = append(out, checkResult{Address: object, Kind: kind, Status: sanitize.Clean(c.Status)})
			}
			continue
		}
		for _, in := range c.Instances {
			if !checkFailed(in.Status) {
				continue
			}
			r := checkResult{Address: sanitize.Clean(in.Address.ToDisplay), Kind: kind, Status: sanitize.Clean(in.Status)}
			if r.Address == "" {
				r.Address = object
			}
			for _, p := range in.Problems {
				if m := sanitize.Clean(p.Message); m != "" {
					r.Problems = append(r.Problems, m)
				}
			}
			out = append(out, r)
		}
	}
	return out
}

func checkFailed(status string) bool {
	return status != "pass" && status != "unknown"
}

type relevance struct {
	byResource map[string][]path
	global     []path
}

func newRelevance(attrs []rawRelevantAttr) relevance {
	rel := relevance{byResource: map[string][]path{}}
	for _, a := range attrs {
		p, ok := toPath(a.Attribute)
		if !ok {
			continue
		}
		p = withoutIndices(p)
		if a.Resource == "" {
			rel.global = append(rel.global, p)
			continue
		}
		rel.byResource[a.Resource] = append(rel.byResource[a.Resource], p)
	}
	return rel
}

func (rel relevance) paths(address string) []path {
	return slices.Concat(rel.global, rel.byResource[address])
}

func buildDrift(r *report, p *rawPlan, o options) {
	rel := newRelevance(p.RelevantAttributes)
	for _, pr := range p.ResourceDrift {
		paths := rel.paths(pr.Address)
		keep := func(at path) bool {
			for _, rp := range paths {
				if at.hasPrefix(rp) {
					return true
				}
			}
			return false
		}
		c := convert(pr, o, keep)
		relevant := len(paths) > 0 && (len(c.Attrs) > 0 || !hasAttrDiff(c.Action))
		if relevant {
			r.Drift = append(r.Drift, c)
			r.Summary.Drift++
			continue
		}
		r.Summary.DriftIgnored++
		if !o.IncludeNoop {
			r.DriftHidden++
			continue
		}
		all := convert(pr, o, nil)
		all.Irrelevant = true
		r.Drift = append(r.Drift, all)
	}
	sortChanges(r.Drift)
	r.Drift, r.DriftOmitted = core.Limit(r.Drift, o.MaxResources)
}

func buildDeferred(r *report, p *rawPlan, o options) {
	for _, d := range p.DeferredChanges {
		rc := d.ResourceChange
		action := mapAction(rc.Change.Actions, rc.Change.Importing != nil)
		isMoved := rc.PreviousAddress != "" && rc.PreviousAddress != rc.Address
		if action == actionNoop && !isMoved {
			continue
		}
		r.Deferred = append(r.Deferred, deferredChange{
			Address: sanitize.Clean(rc.Address), Action: action, Actions: rawActions(rc.Change.Actions, action), Reason: sanitize.Clean(d.Reason),
		})
	}
	slices.SortStableFunc(r.Deferred, func(a, b deferredChange) int {
		return cmp.Or(cmp.Compare(a.Action.rank(), b.Action.rank()), cmp.Compare(a.Address, b.Address))
	})
	r.Summary.Deferred = len(r.Deferred)
	r.Deferred, r.DeferredOmitted = core.Limit(r.Deferred, o.MaxResources)
}

func convertActions(actions []rawAction) []tfdiag.ActionInvocation {
	out := make([]tfdiag.ActionInvocation, 0, len(actions))
	for _, a := range actions {
		out = append(out, convertAction(a))
	}
	return out
}

func convertAction(a rawAction) tfdiag.ActionInvocation {
	inv := tfdiag.ActionInvocation{Address: sanitize.Clean(a.Address)}
	switch {
	case a.LifecycleActionTrigger != nil:
		inv.Event = sanitize.Clean(a.LifecycleActionTrigger.ActionTriggerEvent)
		inv.Resource = sanitize.Clean(a.LifecycleActionTrigger.TriggeringResourceAddress)
	case a.InvokeActionTrigger != nil:
		inv.Invoked = true
	}
	return inv
}

func hasAttrDiff(a action) bool {
	return a == actionUpdate || a.isReplace()
}

func convert(pr rawResource, o options, keep func(path) bool) resourceChange {
	importing := pr.Change.Importing != nil
	action := mapAction(pr.Change.Actions, importing)
	c := resourceChange{
		Address:         sanitize.Clean(pr.Address),
		PreviousAddress: sanitize.Clean(pr.PreviousAddress),
		Deposed:         sanitize.Clean(pr.Deposed),
		Action:          action,
		Actions:         rawActions(pr.Change.Actions, action),
		Importing:       importing,
		Reason:          sanitize.Clean(pr.ActionReason),
	}
	if hcl := generatedConfig(pr.Change.GeneratedConfig); hcl != "" {
		c.GeneratedLines = strings.Count(hcl, "\n") + 1
		if !o.NoValues {
			c.GeneratedConfig = hcl
		}
	}
	if hasAttrDiff(action) {
		c.Attrs, c.ReplacedBy = diffAttrs(pr.Change, !o.NoValues)
	}
	if keep != nil {
		kept := c.Attrs[:0]
		for _, a := range c.Attrs {
			if keep(a.at) {
				kept = append(kept, a)
			}
		}
		c.Attrs = kept
	}
	c.Attrs, c.AttrsOmitted = core.Limit(c.Attrs, o.MaxAttrs)
	return c
}

func generatedConfig(s string) string {
	var lines []string
	for l := range core.SplitLines(s) {
		lines = append(lines, sanitize.Line(l))
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

func count(s *summary, c resourceChange) {
	switch c.Action {
	case actionCreate:
		s.Add++
	case actionUpdate:
		s.Change++
	case actionDelete:
		s.Destroy++
	case actionReplace, actionReplaceCBD:
		s.Add++
		s.Destroy++
		s.Replace++
	case actionForgetThenCreate, actionCreateThenForget:
		s.Add++
		s.Replace++
		s.Forget++
	case actionForget:
		s.Forget++
	case actionRead:
		s.Read++
	case actionNoop:
		s.Noop++
	case actionUnknown:
		s.Unknown++
	case actionImport:
	}
	if c.Importing {
		s.Import++
	}
	if moved(c) {
		s.Move++
	}
}

func moved(c resourceChange) bool {
	return c.PreviousAddress != "" && c.PreviousAddress != c.Address
}

func sortChanges(cs []resourceChange) {
	slices.SortStableFunc(cs, func(a, b resourceChange) int {
		return cmp.Or(cmp.Compare(a.Action.rank(), b.Action.rank()), cmp.Compare(a.Address, b.Address), cmp.Compare(a.Deposed, b.Deposed))
	})
}

func convertOutputs(changes map[string]rawChange, includeNoop bool) []outputChange {
	names := slices.Sorted(maps.Keys(changes))
	out := []outputChange{}
	for _, name := range names {
		ch := changes[name]
		action := mapAction(ch.Actions, false)
		if action == actionNoop && !includeNoop {
			continue
		}
		out = append(out, outputChange{
			Name:      sanitize.Clean(name),
			Action:    action,
			Actions:   rawActions(ch.Actions, action),
			Unknown:   isTrue(ch.AfterUnknown),
			Sensitive: marked(ch.BeforeSensitive) || marked(ch.AfterSensitive),
		})
	}
	return out
}

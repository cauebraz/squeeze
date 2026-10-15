package tfplan

import (
	"sort"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

func conditions(r Report) []string {
	s := r.Summary
	checkBlock, otherCheck := false, false
	for _, c := range r.Checks {
		if c.Kind == "check" {
			checkBlock = true
		} else {
			otherCheck = true
		}
	}
	return core.Conditions(map[string]bool{
		"change":     s.Changed(),
		"check":      len(r.Checks) > 0,
		"deferred":   s.Deferred+s.DeferredActions > 0,
		"destroy":    s.Destroy > 0,
		"drift":      s.Drift > 0,
		"error":      s.Errored,
		"warning":    checkBlock || (otherCheck && !s.Errored),
		"fail":       s.Errored || !s.Complete,
		"forget":     s.Forget > 0,
		"import":     s.Import > 0,
		"incomplete": !s.Complete,
		"move":       s.Move > 0,
		"replace":    s.Replace > 0,
	})
}

func Build(p *Plan, o Options) (Report, error) {
	r := Report{
		Tool:        "terraform",
		ToolVersion: core.Clean(p.TerraformVersion),
		Summary: Summary{
			Errored:  p.Errored,
			Complete: p.Complete == nil || *p.Complete,
		},
		Changes:         []ResourceChange{},
		Drift:           []ResourceChange{},
		Deferred:        []DeferredChange{},
		DeferredActions: []DeferredAction{},
		Actions:         []ActionInvocation{},
		Outputs:         []OutputChange{},
	}
	for _, pr := range p.ResourceChanges {
		c, err := convert(pr, o, nil)
		if err != nil {
			return Report{}, err
		}
		count(&r.Summary, c)
		if c.Action == Noop && !moved(c) && !o.IncludeNoop {
			r.NoopHidden++
			continue
		}
		r.Changes = append(r.Changes, c)
	}
	sortChanges(r.Changes)
	r.Changes, r.Omitted = core.Limit(r.Changes, o.MaxResources)
	if err := buildDrift(&r, p, o); err != nil {
		return Report{}, err
	}
	if err := buildDeferred(&r, p); err != nil {
		return Report{}, err
	}
	r.Actions = convertActions(p.ActionInvocations)
	r.Summary.Actions = len(r.Actions)
	for _, d := range p.DeferredActions {
		r.DeferredActions = append(r.DeferredActions, DeferredAction{ActionInvocation: convertAction(d.ActionInvocation), Reason: core.Clean(d.Reason)})
	}
	r.Summary.DeferredActions = len(r.DeferredActions)
	outputs, err := convertOutputs(p.OutputChanges, o.IncludeNoop)
	if err != nil {
		return Report{}, err
	}
	r.Outputs = outputs
	for _, out := range outputs {
		if out.Action != Noop {
			r.Summary.Outputs++
		}
	}
	r.Checks = convertChecks(p.Checks)
	r.Summary.ChecksFailed = len(r.Checks)
	return r, nil
}

func convertChecks(checks []PlanCheck) []CheckResult {
	out := []CheckResult{}
	for _, c := range checks {
		kind := core.Clean(c.Address.Kind)
		object := core.Clean(c.Address.ToDisplay)
		if len(c.Instances) == 0 {
			if checkFailed(c.Status) {
				out = append(out, CheckResult{Address: object, Kind: kind, Status: core.Clean(c.Status)})
			}
			continue
		}
		for _, in := range c.Instances {
			if !checkFailed(in.Status) {
				continue
			}
			r := CheckResult{Address: core.Clean(in.Address.ToDisplay), Kind: kind, Status: core.Clean(in.Status)}
			if r.Address == "" {
				r.Address = object
			}
			for _, p := range in.Problems {
				if m := core.Clean(p.Message); m != "" {
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

func newRelevance(attrs []PlanRelevantAttr) relevance {
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
	return append(append([]path(nil), rel.global...), rel.byResource[address]...)
}

func buildDrift(r *Report, p *Plan, o Options) error {
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
		c, err := convert(pr, o, keep)
		if err != nil {
			return err
		}
		relevant := len(paths) > 0 && (len(c.Attrs) > 0 || !hasAttrDiff(c.Action))
		if relevant {
			r.Drift = append(r.Drift, c)
			r.Summary.Drift++
			continue
		}
		r.Summary.DriftIgnored++
		if !o.IncludeNoop {
			r.DriftOmitted++
			continue
		}
		all, err := convert(pr, o, nil)
		if err != nil {
			return err
		}
		all.Irrelevant = true
		r.Drift = append(r.Drift, all)
	}
	sortChanges(r.Drift)
	return nil
}

func buildDeferred(r *Report, p *Plan) error {
	for _, d := range p.DeferredChanges {
		rc := d.ResourceChange
		action, err := mapAction(rc.Change.Actions, rc.Change.Importing != nil)
		if err != nil {
			return err
		}
		isMoved := rc.PreviousAddress != "" && rc.PreviousAddress != rc.Address
		if action == Noop && !isMoved {
			continue
		}
		r.Deferred = append(r.Deferred, DeferredChange{Address: core.Clean(rc.Address), Action: action, Reason: core.Clean(d.Reason)})
	}
	sort.SliceStable(r.Deferred, func(i, j int) bool {
		a, b := r.Deferred[i], r.Deferred[j]
		if a.Action.Rank() != b.Action.Rank() {
			return a.Action.Rank() < b.Action.Rank()
		}
		return a.Address < b.Address
	})
	r.Summary.Deferred = len(r.Deferred)
	return nil
}

func convertActions(actions []PlanAction) []ActionInvocation {
	out := make([]ActionInvocation, 0, len(actions))
	for _, a := range actions {
		out = append(out, convertAction(a))
	}
	return out
}

func convertAction(a PlanAction) ActionInvocation {
	inv := ActionInvocation{Address: core.Clean(a.Address)}
	switch {
	case a.LifecycleActionTrigger != nil:
		inv.Event = core.Clean(a.LifecycleActionTrigger.ActionTriggerEvent)
		inv.Resource = core.Clean(a.LifecycleActionTrigger.TriggeringResourceAddress)
	case a.InvokeActionTrigger != nil:
		inv.Invoked = true
	}
	return inv
}

func hasAttrDiff(a Action) bool {
	return a == Update || a.IsReplace()
}

func convert(pr PlanResource, o Options, keep func(path) bool) (ResourceChange, error) {
	importing := pr.Change.Importing != nil
	action, err := mapAction(pr.Change.Actions, importing)
	if err != nil {
		return ResourceChange{}, err
	}
	c := ResourceChange{
		Address:         core.Clean(pr.Address),
		PreviousAddress: core.Clean(pr.PreviousAddress),
		Deposed:         core.Clean(pr.Deposed),
		Action:          action,
		Importing:       importing,
		Reason:          core.Clean(pr.ActionReason),
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
	return c, nil
}

func generatedConfig(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, l := range lines {
		lines[i] = core.Line(l)
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n")
}

func count(s *Summary, c ResourceChange) {
	switch c.Action {
	case Create:
		s.Add++
	case Update:
		s.Change++
	case Delete:
		s.Destroy++
	case Replace, ReplaceCBD:
		s.Add++
		s.Destroy++
		s.Replace++
	case ForgetThenCreate, CreateThenForget:
		s.Add++
		s.Replace++
		s.Forget++
	case Forget:
		s.Forget++
	case Read:
		s.Read++
	case Noop:
		s.Noop++
	case Import:
	}
	if c.Importing {
		s.Import++
	}
	if moved(c) {
		s.Move++
	}
}

func moved(c ResourceChange) bool {
	return c.PreviousAddress != "" && c.PreviousAddress != c.Address
}

func sortChanges(cs []ResourceChange) {
	sort.SliceStable(cs, func(i, j int) bool {
		a, b := cs[i], cs[j]
		if a.Action.Rank() != b.Action.Rank() {
			return a.Action.Rank() < b.Action.Rank()
		}
		if a.Address != b.Address {
			return a.Address < b.Address
		}
		return a.Deposed < b.Deposed
	})
}

func convertOutputs(changes map[string]PlanChange, includeNoop bool) ([]OutputChange, error) {
	names := make([]string, 0, len(changes))
	for name := range changes {
		names = append(names, name)
	}
	sort.Strings(names)
	out := []OutputChange{}
	for _, name := range names {
		ch := changes[name]
		action, err := mapAction(ch.Actions, false)
		if err != nil {
			return nil, err
		}
		if action == Noop && !includeNoop {
			continue
		}
		out = append(out, OutputChange{
			Name:      core.Clean(name),
			Action:    action,
			Unknown:   isTrue(ch.AfterUnknown),
			Sensitive: isTrue(ch.BeforeSensitive) || isTrue(ch.AfterSensitive),
		})
	}
	return out, nil
}

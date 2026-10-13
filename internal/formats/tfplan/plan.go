package tfplan

import (
	"sort"

	"github.com/cauebraz/squeeze/internal/core"
)

func conditions(r Report) []string {
	s := r.Summary
	return core.Conditions(map[string]bool{
		"change":     s.Changed(),
		"deferred":   s.Deferred > 0,
		"destroy":    s.Destroy > 0,
		"drift":      s.Drift > 0,
		"errored":    s.Errored,
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
		Changes:  []ResourceChange{},
		Drift:    []ResourceChange{},
		Deferred: []DeferredChange{},
		Actions:  []ActionInvocation{},
		Outputs:  []OutputChange{},
	}
	for _, pr := range p.ResourceChanges {
		c, err := convert(pr, o, nil)
		if err != nil {
			return Report{}, err
		}
		count(&r.Summary, c)
		if c.Action == Noop && !moved(c) && !o.IncludeNoop {
			continue
		}
		r.Changes = append(r.Changes, c)
	}
	sortChanges(r.Changes)
	if o.MaxResources > 0 && len(r.Changes) > o.MaxResources {
		r.Omitted = len(r.Changes) - o.MaxResources
		r.Changes = r.Changes[:o.MaxResources]
	}
	if err := buildDrift(&r, p, o); err != nil {
		return Report{}, err
	}
	if err := buildDeferred(&r, p); err != nil {
		return Report{}, err
	}
	r.Actions = convertActions(p.ActionInvocations)
	r.Summary.Actions = len(r.Actions)
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
	return r, nil
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
		inv := ActionInvocation{Address: core.Clean(a.Address)}
		switch {
		case a.LifecycleActionTrigger != nil:
			inv.Event = core.Clean(a.LifecycleActionTrigger.ActionTriggerEvent)
			inv.Resource = core.Clean(a.LifecycleActionTrigger.TriggeringResourceAddress)
		case a.InvokeActionTrigger != nil:
			inv.Invoked = true
		}
		out = append(out, inv)
	}
	return out
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
	if o.MaxAttrs > 0 && len(c.Attrs) > o.MaxAttrs {
		c.AttrsOmitted = len(c.Attrs) - o.MaxAttrs
		c.Attrs = c.Attrs[:o.MaxAttrs]
	}
	return c, nil
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

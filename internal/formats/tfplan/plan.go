package tfplan

import "sort"

func Build(p *Plan, o Options) (Report, error) {
	r := Report{
		Tool:        "terraform",
		ToolVersion: p.TerraformVersion,
		Summary: Summary{
			Errored:  p.Errored,
			Complete: p.Complete == nil || *p.Complete,
		},
		Changes: []ResourceChange{},
		Drift:   []ResourceChange{},
		Outputs: []OutputChange{},
	}
	for _, pr := range p.ResourceChanges {
		c, err := convert(pr, o)
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
	for _, pr := range p.ResourceDrift {
		c, err := convert(pr, o)
		if err != nil {
			return Report{}, err
		}
		r.Drift = append(r.Drift, c)
	}
	sortChanges(r.Drift)
	r.Summary.Drift = len(r.Drift)
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

func convert(pr PlanResource, o Options) (ResourceChange, error) {
	importing := pr.Change.Importing != nil
	action, err := mapAction(pr.Change.Actions, importing)
	if err != nil {
		return ResourceChange{}, err
	}
	c := ResourceChange{
		Address:         pr.Address,
		PreviousAddress: pr.PreviousAddress,
		Deposed:         pr.Deposed,
		Action:          action,
		Importing:       importing,
		Reason:          pr.ActionReason,
	}
	switch action {
	case Update, Replace, ReplaceCBD:
		c.Attrs, c.ReplacedBy = diffAttrs(pr.Change, !o.NoValues)
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
		s.Replace++
	case Read:
		s.Read++
	case Forget:
		s.Forget++
	case Noop:
		s.Noop++
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
			Name:      name,
			Action:    action,
			Unknown:   isTrue(ch.AfterUnknown),
			Sensitive: isTrue(ch.BeforeSensitive) || isTrue(ch.AfterSensitive),
		})
	}
	return out, nil
}

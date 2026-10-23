package core

import "slices"

type SeverityOrder []string

func (o SeverityOrder) Rank(s string) int {
	if i := slices.Index(o, s); i >= 0 {
		return i
	}
	return len(o)
}

func (o SeverityOrder) Known(s string) bool {
	return slices.Contains(o, s)
}

func (o SeverityOrder) AtLeast(s, level string) bool {
	return o.Known(s) && o.Rank(s) <= o.Rank(level)
}

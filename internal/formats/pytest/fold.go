package pytest

import "strings"

func signature(f Failure) string {
	if f.Location == "" {
		return ""
	}
	parts := []string{f.Kind, f.When, f.Location}
	for _, l := range f.Lines {
		if strings.HasPrefix(l, ">") || location.MatchString(l) {
			parts = append(parts, l)
		}
	}
	return strings.Join(parts, "\n")
}

func foldFailures(failures []Failure) ([]Failure, int) {
	first := map[string]int{}
	out := failures[:0:0]
	folded := 0
	for _, f := range failures {
		sig := signature(f)
		if i, ok := first[sig]; ok && sig != "" {
			name := f.Test
			if f.Subtest != "" {
				name += " " + f.Subtest
			}
			out[i].Same = append(out[i].Same, name)
			folded++
			continue
		}
		if sig != "" {
			first[sig] = len(out)
		}
		out = append(out, f)
	}
	return out, folded
}

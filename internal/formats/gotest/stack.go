package gotest

import (
	"regexp"
	"strings"
)

var fileLine = regexp.MustCompile(`^\s+\S+\.(go|s):\d+( \+0x[0-9a-f]+)?$`)

func filterStack(lines []string, pkg string) ([]string, int) {
	root, _, _ := strings.Cut(pkg, "/")
	out := make([]string, 0, len(lines))
	dropped := 0
	header := ""
	flush := func() {
		if header != "" {
			out = append(out, header)
			header = ""
		}
	}
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		switch {
		case isGoroutineHeader(l):
			if header != "" {
				dropped++
			}
			header = l
		case i+1 < len(lines) && isFrameStart(l) && fileLine.MatchString(lines[i+1]):
			if userFrame(strings.TrimSpace(l), root) {
				flush()
				out = append(out, l, lines[i+1])
			} else {
				dropped += 2
			}
			i++
		case strings.TrimSpace(l) == "" && header != "":
			dropped++
			header = ""
		default:
			flush()
			out = append(out, l)
		}
	}
	if header != "" {
		dropped++
	}
	return out, dropped
}

func isGoroutineHeader(l string) bool {
	return strings.HasPrefix(l, "goroutine ") && strings.HasSuffix(l, "]:")
}

func isFrameStart(l string) bool {
	t := strings.TrimSpace(l)
	return !fileLine.MatchString(l) && (strings.HasPrefix(t, "created by ") || strings.Contains(t, "("))
}

func userFrame(name, root string) bool {
	name = strings.TrimPrefix(name, "created by ")
	if i := strings.Index(name, " in goroutine "); i >= 0 {
		name = name[:i]
	}
	if strings.HasPrefix(name, "panic(") || strings.HasPrefix(name, "main.main(") {
		return false
	}
	first, _, slash := strings.Cut(name, "/")
	if !slash {
		pkgName, _, _ := strings.Cut(name, ".")
		return pkgName == "main" || pkgName == root
	}
	return strings.Contains(first, ".") || first == root
}

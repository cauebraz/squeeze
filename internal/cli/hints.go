package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

const undefinedFlag = "flag provided but not defined: -"

func unknownFormat(arg string, reg *core.Registry) string {
	if isFile(arg) {
		return fmt.Sprintf("%q is a file; squeeze reads stdin: squeeze < %s", arg, arg)
	}
	msg := fmt.Sprintf("unknown format %q (formats: %s)", arg, strings.Join(reg.Names(), ", "))
	if best := closest(arg, reg.Names()); best != "" {
		msg += fmt.Sprintf("; did you mean %q?", best)
	}
	return msg
}

func unexpectedArgument(arg, command string) string {
	if isFile(arg) {
		return fmt.Sprintf("%q is a file; squeeze reads stdin: %s < %s", arg, command, arg)
	}
	return fmt.Sprintf("unexpected argument %q; input is read from stdin", arg)
}

func flagHint(err error, current core.Format, reg *core.Registry) string {
	name, ok := strings.CutPrefix(err.Error(), undefinedFlag)
	if !ok {
		return ""
	}
	if name == "version" && current != nil {
		return "-version works without a format: squeeze -version"
	}
	var owners []string
	for _, f := range reg.Formats() {
		if current != nil && f.Name() == current.Name() {
			continue
		}
		fs := flag.NewFlagSet(f.Name(), flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		f.Configure(fs)
		if fs.Lookup(name) != nil {
			owners = append(owners, f.Name())
		}
	}
	switch len(owners) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("-%s is a flag of format %s: squeeze %s -%s", name, owners[0], owners[0], name)
	default:
		return fmt.Sprintf("-%s is a flag of formats %s: squeeze <format> -%s", name, strings.Join(owners, ", "), name)
	}
}

func isFile(arg string) bool {
	info, err := os.Stat(arg)
	return err == nil && info.Mode().IsRegular()
}

func closest(word string, candidates []string) string {
	best, bestDist := "", 3
	for _, c := range candidates {
		if d := levenshtein(word, c); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best
}

func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

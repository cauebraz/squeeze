package cli

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/cauebraz/squeeze/internal/core"
)

const undefinedFlag = "flag provided but not defined: -"

var flagErrorNames = strings.NewReplacer(
	"flag needs an argument: -", "",
	"for flag -", "for --",
	"for -", "for --",
	"invalid boolean flag ", "invalid boolean flag --",
)

func flagError(err error, current core.Format, reg *core.Registry) string {
	msg := err.Error()
	if name, ok := strings.CutPrefix(msg, undefinedFlag); ok {
		return unknownFlag(name, current, reg)
	}
	if name, ok := strings.CutPrefix(msg, "flag needs an argument: -"); ok {
		return "--" + name + " needs a value"
	}
	if strings.HasPrefix(msg, "bad flag syntax: ") {
		return msg + " (" + command(current) + " -h lists flags)"
	}
	return flagErrorNames.Replace(msg)
}

func command(f core.Format) string {
	if f == nil {
		return "squeeze"
	}
	return "squeeze " + f.Name()
}

func unknownFlag(name string, current core.Format, reg *core.Registry) string {
	if current == nil {
		return fmt.Sprintf("unknown flag --%s (squeeze -h lists flags)", name)
	}
	prefix := fmt.Sprintf("--%s is not a %s flag", name, current.Name())
	if name == "version" {
		return prefix + " (without a format: squeeze --version)"
	}
	var owners []string
	for _, f := range reg.Formats() {
		if f.Name() == current.Name() {
			continue
		}
		if flags, _ := formatFlags(f); flags.Lookup(name) != nil {
			owners = append(owners, f.Name())
		}
	}
	switch len(owners) {
	case 0:
		return fmt.Sprintf("%s (squeeze %s -h)", prefix, current.Name())
	case 1:
		return fmt.Sprintf("%s (format %s has it: squeeze %s --%s)", prefix, owners[0], owners[0], name)
	default:
		return fmt.Sprintf("%s (formats %s have it: squeeze <format> --%s)", prefix, strings.Join(owners, ", "), name)
	}
}

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

func formatError(f core.Format, err error) string {
	var syntax *json.SyntaxError
	var shape *json.UnmarshalTypeError
	var xmlSyntax *xml.SyntaxError
	msg := err.Error()
	switch {
	case errors.As(err, &syntax):
		msg = fmt.Sprintf("input is not valid JSON (%s at byte %d)", syntax.Error(), syntax.Offset)
	case errors.As(err, &shape):
		msg = fmt.Sprintf("input is JSON of another shape (a JSON %s at byte %d)", shape.Value, shape.Offset)
	case errors.As(err, &xmlSyntax):
		msg = fmt.Sprintf("input is not valid XML (line %d: %s)", xmlSyntax.Line, xmlSyntax.Msg)
	case errors.Is(err, io.ErrUnexpectedEOF):
		msg = "input ends early (unexpected EOF)"
	}
	return fmt.Sprintf("%s: %s; %s reads %s", f.Name(), msg, f.Name(), f.Description())
}

func undetected(head []byte, reg *core.Registry) string {
	if reg.Fallback() == nil {
		return fmt.Sprintf("%v; name it: squeeze <format> (formats: %s)", core.ErrUnknownFormat, strings.Join(reg.Names(), ", "))
	}
	if i := strings.IndexByte(string(head), 0); i >= 0 {
		return fmt.Sprintf("input is binary (NUL byte at offset %d); squeeze reads text or JSON", i)
	}
	return fmt.Sprintf("input is not text (invalid UTF-8 at byte %d); squeeze reads text or JSON, or name a format: squeeze <format> (squeeze -h lists formats)", invalidUTF8(head))
}

func invalidUTF8(b []byte) int {
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			return i
		}
		i += size
	}
	return len(b)
}

func rawDirError(dir string, err error) string {
	verb := "cannot save"
	var mkdir *fs.PathError
	if errors.As(err, &mkdir) && mkdir.Op == "mkdir" {
		verb = "cannot create"
	}
	reason := err
	var link *os.LinkError
	switch {
	case errors.As(err, &mkdir):
		reason = mkdir.Err
	case errors.As(err, &link):
		reason = link.Err
	}
	return fmt.Sprintf("--raw-dir %s: %s: %v", dir, verb, reason)
}

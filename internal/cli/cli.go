package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formats/buildkit"
	"github.com/cauebraz/squeeze/internal/formats/checkov"
	"github.com/cauebraz/squeeze/internal/formats/generic"
	"github.com/cauebraz/squeeze/internal/formats/ghactions"
	"github.com/cauebraz/squeeze/internal/formats/gotest"
	"github.com/cauebraz/squeeze/internal/formats/jest"
	"github.com/cauebraz/squeeze/internal/formats/junit"
	"github.com/cauebraz/squeeze/internal/formats/kubectl"
	"github.com/cauebraz/squeeze/internal/formats/npmaudit"
	"github.com/cauebraz/squeeze/internal/formats/precommit"
	"github.com/cauebraz/squeeze/internal/formats/pytest"
	"github.com/cauebraz/squeeze/internal/formats/tfjson"
	"github.com/cauebraz/squeeze/internal/formats/tflint"
	"github.com/cauebraz/squeeze/internal/formats/tfplan"
	"github.com/cauebraz/squeeze/internal/formats/tfstate"
	"github.com/cauebraz/squeeze/internal/formats/tfvalidate"
	"github.com/cauebraz/squeeze/internal/formats/trivy"
)

var Version = "dev"

const (
	ExitOK            = 0
	ExitError         = 1
	ExitUnknownFormat = 2
	ExitCondition     = 3
	ExitUsage         = 64
)

func DefaultRegistry() *core.Registry {
	return core.NewRegistry(
		tfplan.Format{},
		tfstate.Format{},
		tfvalidate.Format{},
		tflint.Format{},
		ghactions.Format{},
		buildkit.Format{},
		jest.Format{},
		junit.Format{},
		kubectl.Format{},
		npmaudit.Format{},
		precommit.Format{},
		checkov.Format{},
		trivy.Format{},
		pytest.Format{},
		tfjson.Format{},
		gotest.Format{},
	).WithFallback(generic.Format{})
}

type invocation struct {
	format   core.Format
	squeeze  core.Squeezer
	output   string
	rawDir   string
	failOn   []string
	version  bool
	deferred []deferredFlag
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run(args, stdin, stdout, stderr, DefaultRegistry())
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, reg *core.Registry) int {
	inv, code, ok := parse(args, stdout, stderr, reg)
	if !ok {
		return code
	}
	if inv.version {
		fmt.Fprintf(stdout, "squeeze %s\n", version())
		return ExitOK
	}
	input, err := io.ReadAll(stdin)
	if errors.Is(err, errNoPipe) {
		fmt.Fprintf(stderr, "squeeze: %v\n", err)
		return ExitError
	}
	if err != nil {
		fmt.Fprintf(stderr, "squeeze: read stdin: %v\n", err)
		return ExitError
	}
	if len(bytes.TrimSpace(input)) == 0 {
		return noInput(inv, len(input), stdout, stderr)
	}
	if inv.rawDir != "" {
		path, err := saveRaw(inv.rawDir, input)
		if err != nil {
			fmt.Fprintf(stderr, "squeeze: %s\n", rawDirError(inv.rawDir, err))
			return ExitError
		}
		fmt.Fprintf(stderr, "squeeze: raw input saved to %s\n", path)
	}
	if inv.format == nil {
		if code, ok := resolveFormat(&inv, input, reg, stderr); !ok {
			return code
		}
	}
	res, err := inv.squeeze(input)
	if err != nil {
		fmt.Fprintf(stderr, "squeeze: %s\n", formatError(inv.format, err))
		return ExitError
	}
	out := []byte(res.Text)
	if inv.output == "json" {
		out, err = core.Envelope(inv.format.Name(), len(input), res, inv.failOn)
		if err != nil {
			fmt.Fprintf(stderr, "squeeze: render: %v\n", err)
			return ExitError
		}
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "squeeze: write stdout: %v\n", err)
		return ExitError
	}
	if matched := core.Matched(res.Conditions, inv.failOn); len(matched) > 0 {
		fmt.Fprintf(stderr, "squeeze: --fail-on matched: %s\n", strings.Join(matched, ", "))
		return ExitCondition
	}
	return ExitOK
}

func resolveFormat(inv *invocation, input []byte, reg *core.Registry, stderr io.Writer) (int, bool) {
	f, err := reg.Detect(input)
	if err != nil {
		fmt.Fprintf(stderr, "squeeze: %s\n", undetected(core.Head(input), reg))
		return ExitUnknownFormat, false
	}
	if reg.IsFallback(f) {
		fmt.Fprintf(stderr, "squeeze: no format detected; used %s\n", f.Name())
	}
	squeeze, ok := configureDetected(f, inv.deferred, stderr)
	if !ok {
		return ExitUsage, false
	}
	if err := checkConditions(f, inv.failOn); err != nil {
		fmt.Fprintf(stderr, "squeeze: %v\n", err)
		return ExitUsage, false
	}
	inv.format, inv.squeeze = f, squeeze
	return ExitOK, true
}

func checkConditions(f core.Format, failOn []string) error {
	known := f.Conditions()
	for _, name := range failOn {
		if !slices.Contains(known, name) && !slices.Contains(core.SharedConditions(), name) {
			list := conditionList(known)
			if silent := silentConditions(f); len(silent) > 0 {
				list += "; " + strings.Join(silent, ", ") + ": accepted, never present"
			}
			return fmt.Errorf("--fail-on: %s has no condition %q (conditions: %s)", f.Name(), name, list)
		}
	}
	return nil
}

func silentConditions(f core.Format) []string {
	var silent []string
	for _, name := range core.SharedConditions() {
		if !slices.Contains(f.Conditions(), name) {
			silent = append(silent, name)
		}
	}
	return silent
}

func conditionList(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

type listFlag struct{ values *[]string }

func (l listFlag) String() string {
	if l.values == nil {
		return ""
	}
	return strings.Join(*l.values, ",")
}

func (l listFlag) Set(s string) error {
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return errors.New("empty condition name")
		}
		if !slices.Contains(*l.values, part) {
			*l.values = append(*l.values, part)
		}
	}
	return nil
}

func parse(args []string, stdout, stderr io.Writer, reg *core.Registry) (invocation, int, bool) {
	var inv invocation
	args = aliases(args, reg)
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		f, ok := reg.Lookup(args[0])
		if !ok {
			fmt.Fprintf(stderr, "squeeze: %s\n", unknownFormat(args[0], reg))
			return inv, ExitUsage, false
		}
		inv.format = f
		args = args[1:]
	}
	usage, parsing := flagSets(&inv, reg)
	if err := parsing.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			usage.SetOutput(stdout)
			if inv.format == nil {
				rootUsage(stdout, usage, reg)
			} else {
				formatUsage(stdout, usage, inv.format)
			}
			return inv, ExitOK, false
		}
		fmt.Fprintf(stderr, "squeeze: %s\n", flagError(err, inv.format, reg))
		return inv, ExitUsage, false
	}
	if parsing.NArg() > 0 {
		fmt.Fprintf(stderr, "squeeze: %s\n", unexpectedArgument(parsing.Arg(0), command(inv.format)))
		return inv, ExitUsage, false
	}
	if err := validate(inv, reg); err != nil {
		fmt.Fprintf(stderr, "squeeze: %v\n", err)
		return inv, ExitUsage, false
	}
	return inv, ExitOK, true
}

func flagSets(inv *invocation, reg *core.Registry) (usage, parsing *flag.FlagSet) {
	usage = flag.NewFlagSet(command(inv.format), flag.ContinueOnError)
	usage.SetOutput(io.Discard)
	usage.StringVar(&inv.output, "output", "text", "output `format`: text, or json for the envelope with the format's data")
	usage.StringVar(&inv.rawDir, "raw-dir", "", "save the unmodified input to this `dir` and print its path on stderr")
	where := "names in squeeze <format> -h"
	if inv.format != nil {
		where = "names listed below"
	}
	usage.Var(listFlag{&inv.failOn}, "fail-on", "exit 3 when the input has any of these comma-separated `conditions`; "+where)
	if inv.format != nil {
		inv.squeeze = inv.format.Configure(usage)
		return usage, usage
	}
	usage.BoolVar(&inv.version, "version", false, "print the version and exit")
	parsing = flag.NewFlagSet(command(nil), flag.ContinueOnError)
	parsing.SetOutput(io.Discard)
	usage.VisitAll(func(fl *flag.Flag) { parsing.Var(fl.Value, fl.Name, fl.Usage) })
	addFormatFlags(parsing, reg, &inv.deferred)
	return usage, parsing
}

func validate(inv invocation, reg *core.Registry) error {
	if inv.output != "text" && inv.output != "json" {
		return fmt.Errorf("--output must be text or json, got %q", inv.output)
	}
	if inv.format != nil {
		return checkConditions(inv.format, inv.failOn)
	}
	return checkAnyConditions(reg, inv.failOn)
}

func aliases(args []string, reg *core.Registry) []string {
	if len(args) == 0 {
		return args
	}
	if _, ok := reg.Lookup(args[0]); ok {
		return args
	}
	switch args[0] {
	case "help":
		return append(slices.Clone(args[1:]), "-h")
	case "version":
		return append([]string{"--version"}, args[1:]...)
	}
	return args
}

func rootUsage(w io.Writer, fs *flag.FlagSet, reg *core.Registry) {
	fmt.Fprint(w, `squeeze compacts a tool's output for coding agents

Reads a tool's output on stdin and prints the smallest text that keeps every change, failure or decision.
Without a format, squeeze detects it and uses the format's defaults;
text that no format matches goes through the generic format.

Usage:
  tool | squeeze [format] [flags]
  squeeze [format] [flags] < file

Examples:
  go test -json ./... | squeeze
  terraform show -json plan.out | squeeze --fail-on destroy

Formats:
`)
	for _, f := range reg.Formats() {
		fmt.Fprintf(w, "  %-12s %s\n", f.Name(), f.Description())
	}
	fmt.Fprint(w, "\nRun squeeze <format> -h for the format's flags.\n\nFlags:\n")
	printFlags(w, allFlags(fs), allFlags(fs))
}

func formatUsage(w io.Writer, fs *flag.FlagSet, f core.Format) {
	fmt.Fprintf(w, "%s\n\nUsage:\n  tool | squeeze %s [flags]\n  squeeze %s [flags] < file\n\n", f.Description(), f.Name(), f.Name())
	all := allFlags(fs)
	var own, common []*flag.Flag
	for _, fl := range all {
		if slices.Contains(commonFlags, fl.Name) {
			common = append(common, fl)
		} else {
			own = append(own, fl)
		}
	}
	if len(own) > 0 {
		fmt.Fprintf(w, "%s flags:\n", f.Name())
		printFlags(w, own, all)
		fmt.Fprintln(w)
	}
	fmt.Fprint(w, "Common flags:\n")
	printFlags(w, common, all)
	fmt.Fprintf(w, "\nConditions for --fail-on:\n  %s\n", conditionList(f.Conditions()))
	if silent := silentConditions(f); len(silent) > 0 {
		fmt.Fprintf(w, "  %s: accepted, never present for this format\n", strings.Join(silent, ", "))
	}
}

var commonFlags = []string{"fail-on", "output", "raw-dir", "version"}

var placeholders = map[string]string{"fail-on": "<list>", "output": "text|json"}

func allFlags(fs *flag.FlagSet) []*flag.Flag {
	var out []*flag.Flag
	fs.VisitAll(func(fl *flag.Flag) { out = append(out, fl) })
	return out
}

func flagLabel(fl *flag.Flag) string {
	label := "--" + fl.Name
	if isBoolFlag(fl.Value) {
		return label
	}
	if p, ok := placeholders[fl.Name]; ok {
		return label + " " + p
	}
	name, _ := flag.UnquoteUsage(fl)
	return label + " <" + name + ">"
}

func printFlags(w io.Writer, flags, align []*flag.Flag) {
	width := 0
	for _, fl := range align {
		width = max(width, len(flagLabel(fl)))
	}
	for _, fl := range flags {
		_, usage := flag.UnquoteUsage(fl)
		if !isBoolFlag(fl.Value) && fl.DefValue != "" {
			usage += fmt.Sprintf(" (default %q)", fl.DefValue)
		}
		fmt.Fprintf(w, "  %-*s  %s\n", width, flagLabel(fl), usage)
	}
}

func saveRaw(dir string, input []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	sum := sha256.Sum256(input)
	path := filepath.Join(dir, hex.EncodeToString(sum[:])[:12])
	f, err := os.CreateTemp(dir, ".squeeze-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(input)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

const noInputText = "no output: the command printed nothing\n"

func noInput(inv invocation, inputBytes int, stdout, stderr io.Writer) int {
	out := []byte(noInputText)
	if inv.output == "json" {
		name := ""
		if inv.format != nil {
			name = inv.format.Name()
		}
		var err error
		out, err = core.Envelope(name, inputBytes, core.Result{Text: noInputText, Data: struct{}{}}, inv.failOn)
		if err != nil {
			fmt.Fprintf(stderr, "squeeze: render: %v\n", err)
			return ExitError
		}
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "squeeze: write stdout: %v\n", err)
		return ExitError
	}
	return ExitOK
}

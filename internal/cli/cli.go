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
	"github.com/cauebraz/squeeze/internal/formats/checkov"
	"github.com/cauebraz/squeeze/internal/formats/eslint"
	"github.com/cauebraz/squeeze/internal/formats/generic"
	"github.com/cauebraz/squeeze/internal/formats/gotest"
	"github.com/cauebraz/squeeze/internal/formats/pytest"
	"github.com/cauebraz/squeeze/internal/formats/sarif"
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
	return core.NewRegistry(tfplan.Format{}, tfstate.Format{}, tfvalidate.Format{}, tflint.Format{}, eslint.Format{}, sarif.Format{}, checkov.Format{}, trivy.Format{}, pytest.Format{}, tfjson.Format{}, gotest.Format{}).WithFallback(generic.Format{})
}

type invocation struct {
	format  core.Format
	squeeze core.Squeezer
	output  string
	rawDir  string
	failOn  []string
	version bool
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
		fmt.Fprintln(stderr, "squeeze: empty input")
		return ExitError
	}
	if inv.rawDir != "" {
		path, err := saveRaw(inv.rawDir, input)
		if err != nil {
			fmt.Fprintf(stderr, "squeeze: save raw input: %v\n", err)
			return ExitError
		}
		fmt.Fprintf(stderr, "squeeze: raw input saved to %s\n", path)
	}
	if inv.format == nil {
		names := strings.Join(reg.Names(), ", ")
		f, err := reg.Detect(input)
		if err != nil {
			reason := err.Error()
			if reg.Fallback() != nil {
				reason += " and is not text"
			}
			fmt.Fprintf(stderr, "squeeze: %s; name it: squeeze <format> (formats: %s)\n", reason, names)
			return ExitUnknownFormat
		}
		if reg.IsFallback(f) {
			fmt.Fprintf(stderr, "squeeze: no format detected; used %s\n", f.Name())
		}
		inv.format = f
		inv.squeeze = f.Configure(flag.NewFlagSet(f.Name(), flag.ContinueOnError))
		if err := checkConditions(f, inv.failOn); err != nil {
			fmt.Fprintf(stderr, "squeeze: %v\n", err)
			return ExitUsage
		}
	}
	res, err := inv.squeeze(input)
	if err != nil {
		fmt.Fprintf(stderr, "squeeze: %s: %v\n", inv.format.Name(), err)
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

func checkConditions(f core.Format, failOn []string) error {
	known := f.Conditions()
	for _, name := range failOn {
		if !slices.Contains(known, name) && !slices.Contains(core.SharedConditions, name) {
			return fmt.Errorf("--fail-on: %s has no condition %q (conditions: %s)", f.Name(), name, conditionList(known))
		}
	}
	return nil
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
	name := "squeeze"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		f, ok := reg.Lookup(args[0])
		if !ok {
			fmt.Fprintf(stderr, "squeeze: %s\n", unknownFormat(args[0], reg))
			return inv, ExitUsage, false
		}
		inv.format = f
		name += " " + f.Name()
		args = args[1:]
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&inv.output, "output", "text", "output `format`: text, or json for the envelope with the format's data")
	fs.StringVar(&inv.rawDir, "raw-dir", "", "save the unmodified input to this `dir` and print its path on stderr")
	where := "names in squeeze <format> -h"
	if inv.format != nil {
		where = "names listed below"
	}
	fs.Var(listFlag{&inv.failOn}, "fail-on", "exit 3 when the input has any of these comma-separated `conditions`; "+where)
	if inv.format == nil {
		fs.BoolVar(&inv.version, "version", false, "print the version and exit")
	} else {
		inv.squeeze = inv.format.Configure(fs)
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fs.SetOutput(stdout)
			if inv.format == nil {
				rootUsage(stdout, fs, reg)
			} else {
				formatUsage(stdout, fs, inv.format)
			}
			return inv, ExitOK, false
		}
		fmt.Fprintf(stderr, "squeeze: %v\n", err)
		if hint := flagHint(err, inv.format, reg); hint != "" {
			fmt.Fprintf(stderr, "squeeze: %s\n", hint)
		}
		fmt.Fprintf(stderr, "squeeze: run '%s -h' for usage\n", name)
		return inv, ExitUsage, false
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "squeeze: %s\n", unexpectedArgument(fs.Arg(0), name))
		return inv, ExitUsage, false
	}
	if inv.output != "text" && inv.output != "json" {
		fmt.Fprintf(stderr, "squeeze: --output must be text or json, got %q\n", inv.output)
		return inv, ExitUsage, false
	}
	if inv.format != nil {
		if err := checkConditions(inv.format, inv.failOn); err != nil {
			fmt.Fprintf(stderr, "squeeze: %v\n", err)
			return inv, ExitUsage, false
		}
	}
	return inv, ExitOK, true
}

func rootUsage(w io.Writer, fs *flag.FlagSet, reg *core.Registry) {
	fmt.Fprint(w, `usage: squeeze [flags] < input
       squeeze <format> [flags] < input

Reads a tool's output on stdin and prints the smallest text that keeps every change, failure or decision.
Without a format, squeeze detects it and uses the format's defaults;
text that no format matches goes through the generic format.

Formats:
`)
	for _, f := range reg.Formats() {
		fmt.Fprintf(w, "  %-12s %s\n", f.Name(), f.Description())
	}
	fmt.Fprint(w, "\nRun squeeze <format> -h for the format's flags.\n\nFlags:\n")
	fs.PrintDefaults()
}

func formatUsage(w io.Writer, fs *flag.FlagSet, f core.Format) {
	fmt.Fprintf(w, "usage: squeeze %s [flags] < input\n\n%s\n\nFlags:\n", f.Name(), f.Description())
	fs.PrintDefaults()
	fmt.Fprintf(w, "\nConditions for --fail-on:\n  %s\n", conditionList(f.Conditions()))
	var silent []string
	for _, name := range core.SharedConditions {
		if !slices.Contains(f.Conditions(), name) {
			silent = append(silent, name)
		}
	}
	if len(silent) > 0 {
		fmt.Fprintf(w, "  %s: accepted, never present for this format\n", strings.Join(silent, ", "))
	}
}

func saveRaw(dir string, input []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	sum := sha256.Sum256(input)
	path := filepath.Join(dir, hex.EncodeToString(sum[:])[:12])
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return path, nil
	}
	if err != nil {
		return "", err
	}
	if _, err := f.Write(input); err != nil {
		f.Close()
		return "", err
	}
	return path, f.Close()
}

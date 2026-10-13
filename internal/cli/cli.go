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
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formats/tfplan"
)

var Version = "dev"

const (
	ExitOK            = 0
	ExitError         = 1
	ExitUnknownFormat = 2
	ExitUsage         = 64
)

func DefaultRegistry() *core.Registry {
	return core.NewRegistry(tfplan.Format{})
}

type invocation struct {
	format  core.Format
	squeeze core.Squeezer
	output  string
	rawDir  string
	version bool
}

func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run(args, stdin, stdout, stderr, DefaultRegistry())
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, reg *core.Registry) int {
	inv, code, ok := parse(args, stderr, reg)
	if !ok {
		return code
	}
	if inv.version {
		fmt.Fprintf(stdout, "squeeze %s\n", Version)
		return ExitOK
	}
	input, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "squeeze: read stdin: %v\n", err)
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
	if len(bytes.TrimSpace(input)) == 0 {
		fmt.Fprintln(stderr, "squeeze: empty input")
		return ExitError
	}
	if inv.format == nil {
		f, err := reg.Detect(input)
		if err != nil {
			fmt.Fprintf(stderr, "squeeze: %v; name it: squeeze <format> (formats: %s)\n", err, strings.Join(reg.Names(), ", "))
			return ExitUnknownFormat
		}
		inv.format = f
		inv.squeeze = f.Configure(flag.NewFlagSet(f.Name(), flag.ContinueOnError))
	}
	res, err := inv.squeeze(input)
	if err != nil {
		fmt.Fprintf(stderr, "squeeze: %s: %v\n", inv.format.Name(), err)
		return ExitError
	}
	out := []byte(res.Text)
	if inv.output == "json" {
		out, err = core.Envelope(inv.format.Name(), len(input), res)
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

func parse(args []string, stderr io.Writer, reg *core.Registry) (invocation, int, bool) {
	var inv invocation
	name := "squeeze"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		f, ok := reg.Lookup(args[0])
		if !ok {
			fmt.Fprintf(stderr, "squeeze: unknown format %q (formats: %s)\n", args[0], strings.Join(reg.Names(), ", "))
			return inv, ExitUsage, false
		}
		inv.format = f
		name += " " + f.Name()
		args = args[1:]
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&inv.output, "output", "text", "text, or json for the envelope with the format's data")
	fs.StringVar(&inv.rawDir, "raw-dir", "", "save the unmodified input to this directory and print its path on stderr")
	if inv.format == nil {
		fs.BoolVar(&inv.version, "version", false, "print the version and exit")
		fs.Usage = func() { rootUsage(stderr, fs, reg) }
	} else {
		inv.squeeze = inv.format.Configure(fs)
		fs.Usage = func() { formatUsage(stderr, fs, inv.format) }
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return inv, ExitOK, false
		}
		return inv, ExitUsage, false
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "squeeze: unexpected argument %q; input is read from stdin\n", fs.Arg(0))
		return inv, ExitUsage, false
	}
	if inv.output != "text" && inv.output != "json" {
		fmt.Fprintf(stderr, "squeeze: --output must be text or json, got %q\n", inv.output)
		return inv, ExitUsage, false
	}
	return inv, ExitOK, true
}

func rootUsage(w io.Writer, fs *flag.FlagSet, reg *core.Registry) {
	fmt.Fprint(w, `usage: squeeze [flags] < input
       squeeze <format> [flags] < input

Reads a tool's output on stdin and prints the smallest text that keeps every change.
Without a format, squeeze detects it and uses the format's defaults.

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
}

func saveRaw(dir string, input []byte) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	sum := sha256.Sum256(input)
	path := filepath.Join(dir, hex.EncodeToString(sum[:])[:12])
	if err := os.WriteFile(path, input, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

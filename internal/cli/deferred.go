package cli

import (
	"flag"
	"fmt"
	"io"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
)

type deferredFlag struct {
	name, value string
}

type collector struct {
	name   string
	isBool bool
	owners []flag.Value
	into   *[]deferredFlag
}

func (c *collector) String() string { return "" }

func (c *collector) Set(v string) error {
	var first error
	for _, owner := range c.owners {
		err := owner.Set(v)
		if err == nil {
			*c.into = append(*c.into, deferredFlag{c.name, v})
			return nil
		}
		if first == nil {
			first = err
		}
	}
	return first
}

func (c *collector) IsBoolFlag() bool { return c.isBool }

func formatFlags(f core.Format) (*flag.FlagSet, core.Squeezer) {
	fs := flag.NewFlagSet(f.Name(), flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs, f.Configure(fs)
}

func isBoolFlag(v flag.Value) bool {
	b, ok := v.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

func addFormatFlags(fs *flag.FlagSet, reg *core.Registry, into *[]deferredFlag) {
	collectors := map[string]*collector{}
	for _, f := range reg.Formats() {
		tmp, _ := formatFlags(f)
		tmp.VisitAll(func(fl *flag.Flag) {
			if c, ok := collectors[fl.Name]; ok {
				c.owners = append(c.owners, fl.Value)
				return
			}
			if fs.Lookup(fl.Name) == nil {
				c := &collector{fl.Name, isBoolFlag(fl.Value), []flag.Value{fl.Value}, into}
				collectors[fl.Name] = c
				fs.Var(c, fl.Name, fl.Usage)
			}
		})
	}
}

func checkAnyConditions(reg *core.Registry, failOn []string) error {
	for _, name := range failOn {
		if slices.Contains(core.SharedConditions(), name) {
			continue
		}
		if !slices.ContainsFunc(reg.Formats(), func(f core.Format) bool { return slices.Contains(f.Conditions(), name) }) {
			return fmt.Errorf("--fail-on: no format has condition %q; squeeze <format> -h lists a format's conditions", name)
		}
	}
	return nil
}

func configureDetected(f core.Format, flags []deferredFlag, stderr io.Writer) (core.Squeezer, bool) {
	fs, squeeze := formatFlags(f)
	for _, d := range flags {
		if fs.Lookup(d.name) == nil {
			fmt.Fprintf(stderr, "squeeze: --%s is not a flag of the detected format %s (squeeze %s -h)\n", d.name, f.Name(), f.Name())
			return nil, false
		}
		if err := fs.Set(d.name, d.value); err != nil {
			fmt.Fprintf(stderr, "squeeze: invalid value %q for --%s: %v\n", d.value, d.name, err)
			return nil, false
		}
	}
	return squeeze, true
}

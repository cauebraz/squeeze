package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/cauebraz/squeeze/internal/core"
)

type deferredFlag struct {
	name, value string
}

type collector struct {
	name   string
	isBool bool
	into   *[]deferredFlag
}

func (c collector) String() string { return "" }

func (c collector) Set(v string) error {
	*c.into = append(*c.into, deferredFlag{c.name, v})
	return nil
}

func (c collector) IsBoolFlag() bool { return c.isBool }

func isBoolFlag(v flag.Value) bool {
	b, ok := v.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

func addFormatFlags(fs *flag.FlagSet, reg *core.Registry, into *[]deferredFlag) {
	for _, f := range reg.Formats() {
		tmp := flag.NewFlagSet(f.Name(), flag.ContinueOnError)
		tmp.SetOutput(io.Discard)
		f.Configure(tmp)
		tmp.VisitAll(func(fl *flag.Flag) {
			if fs.Lookup(fl.Name) == nil {
				fs.Var(collector{fl.Name, isBoolFlag(fl.Value), into}, fl.Name, fl.Usage)
			}
		})
	}
}

func configureDetected(f core.Format, flags []deferredFlag, stderr io.Writer) (core.Squeezer, bool) {
	fs := flag.NewFlagSet(f.Name(), flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	squeeze := f.Configure(fs)
	for _, d := range flags {
		if fs.Lookup(d.name) == nil {
			fmt.Fprintf(stderr, "squeeze: -%s is not a flag of the detected format %s; run 'squeeze %s -h' for its flags\n", d.name, f.Name(), f.Name())
			return nil, false
		}
		if err := fs.Set(d.name, d.value); err != nil {
			fmt.Fprintf(stderr, "squeeze: invalid value %q for flag -%s: %v\n", d.value, d.name, err)
			return nil, false
		}
	}
	return squeeze, true
}

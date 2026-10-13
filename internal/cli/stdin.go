package cli

import (
	"errors"
	"io"
	"os"
	"runtime/debug"
)

var errNoPipe = errors.New("stdin is a terminal or a device, not a pipe or a file; run `tool | squeeze` or `squeeze < file`")

type noPipe struct{}

func (noPipe) Read([]byte) (int, error) { return 0, errNoPipe }

func Stdin(f *os.File) io.Reader {
	info, err := f.Stat()
	if err == nil && info.Mode()&os.ModeCharDevice != 0 {
		return noPipe{}
	}
	return f
}

func version() string {
	if Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return Version
}

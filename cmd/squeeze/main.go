package main

import (
	"os"

	"github.com/cauebraz/squeeze/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], cli.Stdin(os.Stdin), os.Stdout, os.Stderr))
}

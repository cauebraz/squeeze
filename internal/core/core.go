package core

import (
	"errors"
	"flag"
)

const HeadSize = 64 << 10

var ErrUnknownFormat = errors.New("input matches no known format")

type Result struct {
	Text string
	Data any
}

type Squeezer func(input []byte) (Result, error)

type Format interface {
	Name() string
	Description() string
	Detect(head []byte) bool
	Configure(fs *flag.FlagSet) Squeezer
}

func Head(input []byte) []byte {
	if len(input) > HeadSize {
		return input[:HeadSize]
	}
	return input
}

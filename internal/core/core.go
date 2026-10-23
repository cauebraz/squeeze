package core

import (
	"errors"
	"flag"
	"slices"
)

const HeadSize = 64 << 10

var ErrUnknownFormat = errors.New("input matches no known format")

var sharedConditions = []string{"error", "fail", "warning"}

func SharedConditions() []string { return slices.Clone(sharedConditions) }

type Result struct {
	Text       string
	Data       any
	Conditions []string
}

type Squeezer func(input []byte) (Result, error)

type Format interface {
	Name() string
	Description() string
	Detect(head []byte) bool
	Conditions() []string
	Configure(fs *flag.FlagSet) Squeezer
}

func Head(input []byte) []byte {
	if len(input) > HeadSize {
		return input[:HeadSize]
	}
	return input
}

func Conditions(present map[string]bool) []string {
	out := []string{}
	for name, ok := range present {
		if ok {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

func Matched(conditions, failOn []string) []string {
	out := []string{}
	for _, c := range conditions {
		if slices.Contains(failOn, c) {
			out = append(out, c)
		}
	}
	return out
}

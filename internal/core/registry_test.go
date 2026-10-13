package core

import (
	"bytes"
	"errors"
	"flag"
	"slices"
	"testing"
)

type fake struct {
	name   string
	prefix string
}

func (f fake) Name() string            { return f.name }
func (f fake) Description() string     { return "fake " + f.name }
func (f fake) Detect(head []byte) bool { return bytes.HasPrefix(head, []byte(f.prefix)) }
func (f fake) Conditions() []string    { return nil }
func (f fake) Configure(*flag.FlagSet) Squeezer {
	return func([]byte) (Result, error) { return Result{Text: f.name}, nil }
}

func TestRegistryLookup(t *testing.T) {
	t.Parallel()
	r := NewRegistry(fake{"a", "A"}, fake{"b-2", "B"})
	if f, ok := r.Lookup("b-2"); !ok || f.Name() != "b-2" {
		t.Errorf("Lookup(b-2) = %v, %v", f, ok)
	}
	if _, ok := r.Lookup("c"); ok {
		t.Error("Lookup(c) found a format")
	}
	if got := r.Names(); !slices.Equal(got, []string{"a", "b-2"}) {
		t.Errorf("Names() = %v", got)
	}
	formats := r.Formats()
	formats[0] = fake{"z", ""}
	if r.Formats()[0].Name() != "a" {
		t.Error("Formats() exposes the registry's slice")
	}
}

func TestRegistryDetect(t *testing.T) {
	t.Parallel()
	r := NewRegistry(fake{"first", "X"}, fake{"second", "X"}, fake{"other", "Y"})
	tests := []struct {
		input string
		want  string
		err   error
	}{
		{"X...", "first", nil},
		{"Y...", "other", nil},
		{"Z...", "", ErrUnknownFormat},
		{"", "", ErrUnknownFormat},
	}
	for _, tt := range tests {
		f, err := r.Detect([]byte(tt.input))
		if !errors.Is(err, tt.err) {
			t.Errorf("Detect(%q) error = %v, want %v", tt.input, err, tt.err)
			continue
		}
		if err == nil && f.Name() != tt.want {
			t.Errorf("Detect(%q) = %s, want %s", tt.input, f.Name(), tt.want)
		}
	}
}

func TestRegistryDetectSeesOnlyHead(t *testing.T) {
	t.Parallel()
	r := NewRegistry(fake{"tail", string(make([]byte, HeadSize)) + "T"})
	if _, err := r.Detect(append(make([]byte, HeadSize), 'T')); !errors.Is(err, ErrUnknownFormat) {
		t.Errorf("Detect() matched past HeadSize, error = %v", err)
	}
}

func TestNewRegistryPanics(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		formats []Format
	}{
		{"empty name", []Format{fake{"", ""}}},
		{"flag-like name", []Format{fake{"-x", ""}}},
		{"leading digit", []Format{fake{"1a", ""}}},
		{"uppercase", []Format{fake{"TF", ""}}},
		{"space", []Format{fake{"a b", ""}}},
		{"duplicate", []Format{fake{"a", ""}, fake{"a", ""}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if recover() == nil {
					t.Error("NewRegistry() did not panic")
				}
			}()
			NewRegistry(tt.formats...)
		})
	}
}

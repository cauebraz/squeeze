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

type marker struct {
	name, key string
}

func (m marker) Name() string        { return m.name }
func (m marker) Description() string { return "marker " + m.name }
func (m marker) Detect(input []byte) bool {
	return bytes.Contains(input, []byte(m.key))
}
func (m marker) Conditions() []string { return nil }
func (m marker) Configure(*flag.FlagSet) Squeezer {
	return func([]byte) (Result, error) { return Result{Text: m.name}, nil }
}

func TestRegistryDetectJSONPastHead(t *testing.T) {
	t.Parallel()
	r := NewRegistry(marker{"plan", `"resource_changes"`}).WithFallback(marker{"text", ""})
	pad := bytes.Repeat([]byte(" "), HeadSize)
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{"object past head", slices.Concat([]byte(`{"v":"`), pad, []byte(`","resource_changes":[]}`)), "plan"},
		{"array past head", slices.Concat([]byte(` [{"v":"`), pad, []byte(`","resource_changes":[]}]`)), "plan"},
		{"text past head", slices.Concat([]byte("log "), pad, []byte(`"resource_changes"`)), "text"},
		{"object without marker", slices.Concat([]byte(`{"v":"`), pad, []byte(`"}`)), "text"},
	}
	for _, tt := range tests {
		f, err := r.Detect(tt.input)
		if err != nil || f.Name() != tt.want {
			t.Errorf("%s: Detect() = %v, %v; want %s", tt.name, f, err, tt.want)
		}
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

func TestRegistryFallback(t *testing.T) {
	t.Parallel()
	r := NewRegistry(fake{"a", "A"}).WithFallback(fake{"rest", "R"})
	if got := r.Names(); !slices.Equal(got, []string{"a", "rest"}) {
		t.Errorf("Names() = %v", got)
	}
	if f := r.Fallback(); f == nil || f.Name() != "rest" {
		t.Errorf("Fallback() = %v", f)
	}
	if f, ok := r.Lookup("rest"); !ok || f.Name() != "rest" {
		t.Errorf("Lookup(rest) = %v, %v", f, ok)
	}
	tests := []struct {
		input string
		want  string
		err   error
	}{
		{"A...", "a", nil},
		{"R...", "rest", nil},
		{"Z...", "", ErrUnknownFormat},
	}
	for _, tt := range tests {
		f, err := r.Detect([]byte(tt.input))
		if !errors.Is(err, tt.err) || (err == nil && f.Name() != tt.want) {
			t.Errorf("Detect(%q) = %v, %v; want %s, %v", tt.input, f, err, tt.want, tt.err)
		}
	}
	first := NewRegistry(fake{"rest", "A"}).WithFallback(fake{"later", "A"})
	if f, _ := first.Detect([]byte("A")); f.Name() != "rest" {
		t.Errorf("fallback detected before a regular format: %s", f.Name())
	}
	if NewRegistry(fake{"a", "A"}).Fallback() != nil {
		t.Error("Fallback() without WithFallback is not nil")
	}
}

func TestRegistryFallbackPanics(t *testing.T) {
	t.Parallel()
	for name, build := range map[string]func(){
		"twice":     func() { NewRegistry().WithFallback(fake{"a", ""}).WithFallback(fake{"b", ""}) },
		"duplicate": func() { NewRegistry(fake{"a", ""}).WithFallback(fake{"a", ""}) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if recover() == nil {
					t.Error("WithFallback() did not panic")
				}
			}()
			build()
		})
	}
}

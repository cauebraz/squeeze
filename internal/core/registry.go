package core

import (
	"bytes"
	"fmt"
)

type Registry struct {
	formats  []Format
	byName   map[string]Format
	fallback Format
}

func NewRegistry(formats ...Format) *Registry {
	r := &Registry{byName: make(map[string]Format, len(formats))}
	for _, f := range formats {
		r.add(f)
	}
	return r
}

func (r *Registry) add(f Format) {
	name := f.Name()
	if !validName(name) {
		panic(fmt.Sprintf("core: invalid format name %q", name))
	}
	if _, dup := r.byName[name]; dup {
		panic(fmt.Sprintf("core: duplicate format %q", name))
	}
	r.formats = append(r.formats, f)
	r.byName[name] = f
}

func (r *Registry) WithFallback(f Format) *Registry {
	if r.fallback != nil {
		panic("core: registry already has a fallback")
	}
	r.add(f)
	r.fallback = f
	return r
}

func (r *Registry) Fallback() Format {
	return r.fallback
}

func (r *Registry) IsFallback(f Format) bool {
	return r.fallback != nil && f.Name() == r.fallback.Name()
}

func validName(name string) bool {
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for _, c := range name {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}

func (r *Registry) Lookup(name string) (Format, bool) {
	f, ok := r.byName[name]
	return f, ok
}

func (r *Registry) Detect(input []byte) (Format, error) {
	head := Head(input)
	if f := r.detect(head); f != nil {
		return f, nil
	}
	if len(input) > len(head) && jsonDocument(head) {
		if f := r.detect(input); f != nil {
			return f, nil
		}
	}
	if r.fallback != nil && r.fallback.Detect(head) {
		return r.fallback, nil
	}
	return nil, ErrUnknownFormat
}

func (r *Registry) detect(input []byte) Format {
	for _, f := range r.formats {
		if !r.IsFallback(f) && f.Detect(input) {
			return f
		}
	}
	return nil
}

func jsonDocument(head []byte) bool {
	trimmed := bytes.TrimLeft(head, " \t\r\n")
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[')
}

func (r *Registry) Formats() []Format {
	return append([]Format(nil), r.formats...)
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.formats))
	for _, f := range r.formats {
		names = append(names, f.Name())
	}
	return names
}

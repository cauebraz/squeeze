package core

import "fmt"

type Registry struct {
	formats []Format
	byName  map[string]Format
}

func NewRegistry(formats ...Format) *Registry {
	r := &Registry{byName: make(map[string]Format, len(formats))}
	for _, f := range formats {
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
	return r
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
	for _, f := range r.formats {
		if f.Detect(head) {
			return f, nil
		}
	}
	return nil, ErrUnknownFormat
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

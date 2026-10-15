package kubectl

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrNotKubectl = errors.New("input is not a Kubernetes object or list")

type Meta struct {
	Name              string `json:"name"`
	Namespace         string `json:"namespace"`
	DeletionTimestamp string `json:"deletionTimestamp"`
}

type Raw struct {
	Kind   string
	Meta   Meta
	Fields map[string]json.RawMessage
}

func text(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

func Parse(input []byte) ([]Raw, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(input, &top); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if top == nil || text(top["apiVersion"]) == "" || text(top["kind"]) == "" {
		return nil, ErrNotKubectl
	}
	var out []Raw
	if err := flatten(top, "", &out); err != nil {
		return nil, err
	}
	return out, nil
}

func flatten(fields map[string]json.RawMessage, defaultKind string, out *[]Raw) error {
	kind := text(fields["kind"])
	if kind == "" {
		kind = defaultKind
	}
	if kind == "" {
		return ErrNotKubectl
	}
	if items, ok := fields["items"]; ok && strings.HasSuffix(kind, "List") {
		var list []map[string]json.RawMessage
		if err := json.Unmarshal(items, &list); err != nil {
			return fmt.Errorf("decode %s items: %w", kind, err)
		}
		for _, item := range list {
			if item == nil {
				return ErrNotKubectl
			}
			if err := flatten(item, strings.TrimSuffix(kind, "List"), out); err != nil {
				return err
			}
		}
		return nil
	}
	r := Raw{Kind: kind, Fields: fields}
	if m, ok := fields["metadata"]; ok {
		if err := json.Unmarshal(m, &r.Meta); err != nil {
			return fmt.Errorf("decode %s metadata: %w", kind, err)
		}
	}
	*out = append(*out, r)
	return nil
}

func (r Raw) decode(field string, v any) error {
	raw, ok := r.Fields[field]
	if !ok {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("decode %s %s %s: %w", r.Kind, r.Meta.Name, field, err)
	}
	return nil
}

func (r Raw) keys(fields ...string) []string {
	seen := map[string]bool{}
	var keys []string
	for _, f := range fields {
		var m map[string]json.RawMessage
		if json.Unmarshal(r.Fields[f], &m) != nil {
			continue
		}
		for k := range m {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	return keys
}

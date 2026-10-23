package kubectl

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/logscan"
)

var errNotKubectl = errors.New("input is not a Kubernetes object or list")

type rawMeta struct {
	Name              string `json:"name"`
	Namespace         string `json:"namespace"`
	DeletionTimestamp string `json:"deletionTimestamp"`
}

type rawObject struct {
	Kind   string
	Meta   rawMeta
	Fields map[string]json.RawMessage
	Doc    json.RawMessage
}

func jsonString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return ""
	}
	return s
}

type document struct {
	Objects  []rawObject
	Trailing []logscan.FlaggedLine
}

func parse(input []byte) (document, error) {
	lead, body := logscan.LeadingLines(input)
	var doc json.RawMessage
	rest, err := core.DecodeJSON(body, &doc)
	if err != nil {
		return document{}, fmt.Errorf("decode: %w", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(doc, &top); err != nil {
		return document{}, fmt.Errorf("decode: %w", err)
	}
	if top == nil || jsonString(top["apiVersion"]) == "" || jsonString(top["kind"]) == "" {
		return document{}, errNotKubectl
	}
	var out []rawObject
	if err := flatten(doc, top, "", &out); err != nil {
		return document{}, err
	}
	return document{Objects: out, Trailing: append(logscan.FlaggedLines(lead), logscan.FlaggedLines(rest)...)}, nil
}

func flatten(doc json.RawMessage, fields map[string]json.RawMessage, defaultKind string, out *[]rawObject) error {
	kind := jsonString(fields["kind"])
	if kind == "" {
		kind = defaultKind
	}
	if kind == "" {
		return errNotKubectl
	}
	if items, ok := fields["items"]; ok && strings.HasSuffix(kind, "List") {
		var list []json.RawMessage
		if err := json.Unmarshal(items, &list); err != nil {
			return fmt.Errorf("decode %s items: %w", kind, err)
		}
		for _, item := range list {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(item, &fields); err != nil {
				return fmt.Errorf("decode %s items: %w", kind, err)
			}
			if fields == nil {
				return errNotKubectl
			}
			if err := flatten(item, fields, strings.TrimSuffix(kind, "List"), out); err != nil {
				return err
			}
		}
		return nil
	}
	r := rawObject{Kind: kind, Fields: fields, Doc: doc}
	if m, ok := fields["metadata"]; ok {
		if err := json.Unmarshal(m, &r.Meta); err != nil {
			return fmt.Errorf("decode %s metadata: %w", kind, err)
		}
	}
	*out = append(*out, r)
	return nil
}

func (r rawObject) decode(field string, v any) error {
	raw, ok := r.Fields[field]
	if !ok {
		return nil
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("decode %s %s %s: %w", r.Kind, r.Meta.Name, field, err)
	}
	return nil
}

func (r rawObject) keys(fields ...string) []string {
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

package tfstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/cauebraz/squeeze/internal/core"
)

var (
	ErrNotObject        = errors.New("not a JSON object of outputs")
	ErrMissingSensitive = errors.New("missing sensitive")
)

type RawOutput struct {
	Sensitive *bool           `json:"sensitive"`
	Type      json.RawMessage `json:"type"`
	Value     json.RawMessage `json:"value"`
}

type Output struct {
	Name      string          `json:"name"`
	Sensitive bool            `json:"sensitive"`
	Type      json.RawMessage `json:"type,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
}

type Report struct {
	Tool        string   `json:"tool"`
	ToolVersion string   `json:"tool_version"`
	Total       int      `json:"total"`
	Sensitive   int      `json:"sensitive"`
	Omitted     int      `json:"outputs_omitted"`
	Outputs     []Output `json:"outputs"`
}

func Parse(input []byte) (map[string]RawOutput, error) {
	var outputs map[string]RawOutput
	if err := json.NewDecoder(bytes.NewReader(input)).Decode(&outputs); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if outputs == nil {
		return nil, ErrNotObject
	}
	for name, o := range outputs {
		if o.Sensitive == nil {
			return nil, fmt.Errorf("output %q: %w", name, ErrMissingSensitive)
		}
	}
	return outputs, nil
}

func Build(raw map[string]RawOutput, o Options) Report {
	names := make([]string, 0, len(raw))
	for name := range raw {
		names = append(names, name)
	}
	sort.Strings(names)
	r := Report{Tool: "terraform", Total: len(names), Outputs: []Output{}}
	for _, name := range names {
		ro := raw[name]
		out := Output{Name: name, Sensitive: *ro.Sensitive, Type: compact(ro.Type)}
		if out.Sensitive {
			r.Sensitive++
		} else {
			out.Value = compact(ro.Value)
			if out.Value == nil {
				out.Value = json.RawMessage("null")
			}
		}
		r.Outputs = append(r.Outputs, out)
	}
	r.Outputs, r.Omitted = core.Limit(r.Outputs, o.MaxOutputs)
	return r
}

func compact(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return nil
	}
	return b.Bytes()
}

package tfstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/logscan"
	"github.com/cauebraz/squeeze/internal/sanitize"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

var (
	errNotObject        = errors.New("not a JSON object of outputs")
	errMissingSensitive = errors.New("missing sensitive")
)

type rawOutput struct {
	Sensitive *bool           `json:"sensitive"`
	Type      json.RawMessage `json:"type"`
	Value     json.RawMessage `json:"value"`
}

type rawState struct {
	FormatVersion    string `json:"format_version"`
	TerraformVersion string `json:"terraform_version"`
	Values           *struct {
		Outputs    map[string]rawOutput `json:"outputs"`
		RootModule rawModule            `json:"root_module"`
	} `json:"values"`
}

type rawModule struct {
	Resources    []rawResource `json:"resources"`
	ChildModules []rawModule   `json:"child_modules"`
}

type rawResource struct {
	Address    string `json:"address"`
	Mode       string `json:"mode"`
	Tainted    bool   `json:"tainted"`
	DeposedKey string `json:"deposed_key"`
}

type document struct {
	State    *rawState
	Show     *rawShow
	Outputs  map[string]rawOutput
	Trailing []logscan.FlaggedLine
}

type output struct {
	Name      string          `json:"name"`
	Sensitive bool            `json:"sensitive"`
	Type      json.RawMessage `json:"type,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
}

type resource struct {
	Address    string `json:"address"`
	Mode       string `json:"mode"`
	Tainted    bool   `json:"tainted,omitempty"`
	DeposedKey string `json:"deposed_key,omitempty"`
}

type report struct {
	Tool        string              `json:"tool"`
	ToolVersion string              `json:"tool_version"`
	Source      source              `json:"source"`
	Total       int                 `json:"total"`
	Sensitive   int                 `json:"sensitive"`
	Omitted     int                 `json:"outputs_omitted"`
	Outputs     []output            `json:"outputs"`
	State       *state              `json:"state,omitempty"`
	Resource    *instance           `json:"resource,omitempty"`
	Errors      int                 `json:"errors,omitempty"`
	Warnings    int                 `json:"warnings,omitempty"`
	Diagnostics []tfdiag.Diagnostic `json:"diagnostics,omitempty"`
}

type source string

const (
	sourceOutput   source = "output"
	sourceState    source = "state"
	sourceResource source = "resource"
)

type state struct {
	Managed          int        `json:"managed"`
	DataSources      int        `json:"data_sources"`
	Tainted          int        `json:"tainted"`
	Deposed          int        `json:"deposed"`
	ResourcesOmitted int        `json:"resources_omitted"`
	Resources        []resource `json:"resources"`
}

func parse(input []byte) (document, error) {
	var top map[string]json.RawMessage
	rest, err := core.DecodeJSON(input, &top)
	if err != nil {
		return document{}, fmt.Errorf("decode: %w", err)
	}
	in, err := parseDocument(input[:len(input)-len(rest)], top)
	if err != nil {
		return document{}, err
	}
	in.Trailing = logscan.FlaggedLines(rest)
	return in, nil
}

func parseDocument(input []byte, top map[string]json.RawMessage) (document, error) {
	if top == nil {
		return document{}, errNotObject
	}
	if v, ok := top["format_version"]; ok && bytes.HasPrefix(v, []byte(`"`)) {
		if _, ok := top["resource"]; ok {
			return parseShow(input)
		}
		return parseState(input)
	}
	var outputs map[string]rawOutput
	if err := json.Unmarshal(input, &outputs); err != nil {
		return document{}, fmt.Errorf("decode: %w", err)
	}
	if err := checkOutputs(outputs); err != nil {
		return document{}, err
	}
	return document{Outputs: outputs}, nil
}

func parseState(input []byte) (document, error) {
	var s rawState
	if err := json.NewDecoder(bytes.NewReader(input)).Decode(&s); err != nil {
		return document{}, fmt.Errorf("decode: %w", err)
	}
	if err := tfdiag.CheckFormatVersion(s.FormatVersion); err != nil {
		return document{}, err
	}
	if s.Values != nil {
		if err := checkOutputs(s.Values.Outputs); err != nil {
			return document{}, err
		}
	}
	return document{State: &s}, nil
}

func checkOutputs(outputs map[string]rawOutput) error {
	for _, name := range slices.Sorted(maps.Keys(outputs)) {
		if outputs[name].Sensitive == nil {
			return fmt.Errorf("output %q: %w", name, errMissingSensitive)
		}
	}
	return nil
}

func build(in document, o options) report {
	r := report{Tool: "terraform", Source: sourceOutput}
	if in.Show != nil {
		buildShow(&r, in.Show, in.Trailing)
		r.Outputs = []output{}
		return r
	}
	r.Diagnostics = trailing(nil, in.Trailing)
	r.Errors, r.Warnings = tfdiag.Count(r.Diagnostics)
	outputs := in.Outputs
	if in.State != nil {
		r.Source = sourceState
		r.ToolVersion = sanitize.Clean(in.State.TerraformVersion)
		s := &state{Resources: []resource{}}
		if v := in.State.Values; v != nil {
			outputs = v.Outputs
			walk(s, v.RootModule)
		}
		s.Resources, s.ResourcesOmitted = core.Limit(s.Resources, o.MaxResources)
		r.State = s
	}
	r.Outputs = buildOutputs(&r, outputs)
	r.Outputs, r.Omitted = core.Limit(r.Outputs, o.MaxOutputs)
	return r
}

func trailing(ds []tfdiag.Diagnostic, lines []logscan.FlaggedLine) []tfdiag.Diagnostic {
	ds = append(ds, tfdiag.FromLines(lines, "")...)
	if len(ds) == 0 {
		return nil
	}
	return tfdiag.Normalize(ds)
}

func walk(r *state, m rawModule) {
	for _, raw := range m.Resources {
		res := resource{Address: sanitize.Clean(raw.Address), Mode: sanitize.Clean(raw.Mode), Tainted: raw.Tainted, DeposedKey: sanitize.Clean(raw.DeposedKey)}
		switch {
		case res.DeposedKey != "":
			r.Deposed++
		case res.Mode == "data":
			r.DataSources++
		default:
			r.Managed++
		}
		if res.Tainted {
			r.Tainted++
		}
		r.Resources = append(r.Resources, res)
	}
	for _, child := range m.ChildModules {
		walk(r, child)
	}
}

func buildOutputs(r *report, raw map[string]rawOutput) []output {
	names := slices.Sorted(maps.Keys(raw))
	r.Total = len(names)
	out := []output{}
	for _, name := range names {
		ro := raw[name]
		o := output{Name: name, Sensitive: *ro.Sensitive, Type: compact(ro.Type)}
		if o.Sensitive {
			r.Sensitive++
		} else {
			o.Value = compact(ro.Value)
			if o.Value == nil {
				o.Value = json.RawMessage("null")
			}
		}
		out = append(out, o)
	}
	return out
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

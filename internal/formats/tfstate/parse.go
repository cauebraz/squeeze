package tfstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
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

type RawState struct {
	FormatVersion    string `json:"format_version"`
	TerraformVersion string `json:"terraform_version"`
	Values           *struct {
		Outputs    map[string]RawOutput `json:"outputs"`
		RootModule RawModule            `json:"root_module"`
	} `json:"values"`
}

type RawModule struct {
	Resources    []RawResource `json:"resources"`
	ChildModules []RawModule   `json:"child_modules"`
}

type RawResource struct {
	Address    string `json:"address"`
	Mode       string `json:"mode"`
	Tainted    bool   `json:"tainted"`
	DeposedKey string `json:"deposed_key"`
}

type Input struct {
	State   *RawState
	Show    *RawShow
	Outputs map[string]RawOutput
}

type Output struct {
	Name      string          `json:"name"`
	Sensitive bool            `json:"sensitive"`
	Type      json.RawMessage `json:"type,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
}

type Resource struct {
	Address    string `json:"address"`
	Mode       string `json:"mode"`
	Tainted    bool   `json:"tainted,omitempty"`
	DeposedKey string `json:"deposed_key,omitempty"`
}

type Report struct {
	Tool        string              `json:"tool"`
	ToolVersion string              `json:"tool_version"`
	Source      string              `json:"source"`
	Total       int                 `json:"total"`
	Sensitive   int                 `json:"sensitive"`
	Omitted     int                 `json:"outputs_omitted"`
	Outputs     []Output            `json:"outputs"`
	State       *State              `json:"state,omitempty"`
	Resource    *Instance           `json:"resource,omitempty"`
	Errors      int                 `json:"errors,omitempty"`
	Warnings    int                 `json:"warnings,omitempty"`
	Diagnostics []tfdiag.Diagnostic `json:"diagnostics,omitempty"`
}

type State struct {
	Managed          int        `json:"managed"`
	DataSources      int        `json:"data_sources"`
	Tainted          int        `json:"tainted"`
	Deposed          int        `json:"deposed"`
	ResourcesOmitted int        `json:"resources_omitted"`
	Resources        []Resource `json:"resources"`
}

func Parse(input []byte) (Input, error) {
	var top map[string]json.RawMessage
	if err := json.NewDecoder(bytes.NewReader(input)).Decode(&top); err != nil {
		return Input{}, fmt.Errorf("decode: %w", err)
	}
	if top == nil {
		return Input{}, ErrNotObject
	}
	if v, ok := top["format_version"]; ok && bytes.HasPrefix(v, []byte(`"`)) {
		if _, ok := top["resource"]; ok {
			return parseShow(input)
		}
		return parseState(input)
	}
	var outputs map[string]RawOutput
	if err := json.Unmarshal(input, &outputs); err != nil {
		return Input{}, fmt.Errorf("decode: %w", err)
	}
	if err := checkOutputs(outputs); err != nil {
		return Input{}, err
	}
	return Input{Outputs: outputs}, nil
}

func parseState(input []byte) (Input, error) {
	var s RawState
	if err := json.NewDecoder(bytes.NewReader(input)).Decode(&s); err != nil {
		return Input{}, fmt.Errorf("decode: %w", err)
	}
	if major, _, _ := strings.Cut(s.FormatVersion, "."); major != "1" {
		return Input{}, fmt.Errorf("unsupported format_version %q", s.FormatVersion)
	}
	if s.Values != nil {
		if err := checkOutputs(s.Values.Outputs); err != nil {
			return Input{}, err
		}
	}
	return Input{State: &s}, nil
}

func checkOutputs(outputs map[string]RawOutput) error {
	for name, o := range outputs {
		if o.Sensitive == nil {
			return fmt.Errorf("output %q: %w", name, ErrMissingSensitive)
		}
	}
	return nil
}

func Build(in Input, o Options) Report {
	r := Report{Tool: "terraform", Source: "output"}
	if in.Show != nil {
		buildShow(&r, in.Show)
		r.Outputs = []Output{}
		return r
	}
	outputs := in.Outputs
	if in.State != nil {
		r.Source = "state"
		r.ToolVersion = core.Clean(in.State.TerraformVersion)
		s := &State{Resources: []Resource{}}
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

func walk(r *State, m RawModule) {
	for _, raw := range m.Resources {
		res := Resource{Address: core.Line(raw.Address), Mode: core.Clean(raw.Mode), Tainted: raw.Tainted, DeposedKey: core.Clean(raw.DeposedKey)}
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

func buildOutputs(r *Report, raw map[string]RawOutput) []Output {
	names := make([]string, 0, len(raw))
	for name := range raw {
		names = append(names, name)
	}
	sort.Strings(names)
	r.Total = len(names)
	out := []Output{}
	for _, name := range names {
		ro := raw[name]
		o := Output{Name: name, Sensitive: *ro.Sensitive, Type: compact(ro.Type)}
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

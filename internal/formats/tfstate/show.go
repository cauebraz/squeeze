package tfstate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const sensitiveText = "(sensitive)"

type RawShow struct {
	FormatVersion string       `json:"format_version"`
	Resource      *RawInstance `json:"resource"`
	Diagnostics   []tfdiag.Raw `json:"diagnostics"`
}

type RawInstance struct {
	Address         string          `json:"address"`
	Mode            string          `json:"mode"`
	Type            string          `json:"type"`
	ProviderName    string          `json:"provider_name"`
	Values          json.RawMessage `json:"values"`
	SensitiveValues json.RawMessage `json:"sensitive_values"`
	Tainted         bool            `json:"tainted"`
}

type Instance struct {
	Address      string      `json:"address"`
	Mode         string      `json:"mode"`
	Type         string      `json:"type"`
	ProviderName string      `json:"provider_name"`
	Tainted      bool        `json:"tainted,omitempty"`
	Sensitive    int         `json:"sensitive"`
	Null         int         `json:"null"`
	Attributes   []Attribute `json:"attributes"`
}

type Attribute struct {
	Name            string          `json:"name"`
	Sensitive       bool            `json:"sensitive,omitempty"`
	Value           json.RawMessage `json:"value,omitempty"`
	SensitiveValues json.RawMessage `json:"sensitive_values,omitempty"`
	Text            string          `json:"-"`
}

func parseShow(input []byte) (Input, error) {
	var s RawShow
	if err := json.NewDecoder(bytes.NewReader(input)).Decode(&s); err != nil {
		return Input{}, fmt.Errorf("decode: %w", err)
	}
	if major, _, _ := strings.Cut(s.FormatVersion, "."); major != "1" {
		return Input{}, fmt.Errorf("unsupported format_version %q", s.FormatVersion)
	}
	if r := s.Resource; r != nil && !isNull(r.Values) {
		var vals map[string]json.RawMessage
		if err := json.Unmarshal(r.Values, &vals); err != nil {
			return Input{}, fmt.Errorf("decode values: %w", err)
		}
		if len(r.SensitiveValues) == 0 {
			return Input{}, fmt.Errorf("resource %q: %w", core.Line(r.Address), ErrMissingSensitive)
		}
	}
	return Input{Show: &s}, nil
}

func buildShow(r *Report, s *RawShow, lines []core.FlaggedLine) {
	r.Source = "resource"
	ds := make([]tfdiag.Diagnostic, 0, len(s.Diagnostics))
	for _, raw := range s.Diagnostics {
		ds = append(ds, tfdiag.Convert(raw))
	}
	r.Diagnostics = trailing(ds, lines)
	r.Errors, r.Warnings = tfdiag.Count(r.Diagnostics)
	raw := s.Resource
	if raw == nil || (raw.Address == "" && isNull(raw.Values)) {
		return
	}
	in := &Instance{
		Address:      core.Line(raw.Address),
		Mode:         core.Clean(raw.Mode),
		Type:         core.Clean(raw.Type),
		ProviderName: core.Clean(raw.ProviderName),
		Tainted:      raw.Tainted,
		Attributes:   []Attribute{},
	}
	attrs := attributes(raw.Values, raw.SensitiveValues)
	for _, a := range attrs {
		switch {
		case a.Sensitive || a.SensitiveValues != nil:
			in.Sensitive++
		case string(a.Value) == "null":
			in.Null++
		}
	}
	in.Attributes = attrs
	r.Resource = in
}

func attributes(values, mask json.RawMessage) []Attribute {
	var vals map[string]json.RawMessage
	_ = json.Unmarshal(values, &vals)
	masks := map[string]json.RawMessage{}
	if err := json.Unmarshal(mask, &masks); err != nil {
		masks = nil
		if !clean(mask) {
			masks = map[string]json.RawMessage{}
			for name := range vals {
				masks[name] = json.RawMessage("true")
			}
		}
	}
	names := slices.Sorted(maps.Keys(vals))
	out := make([]Attribute, 0, len(names))
	for _, name := range names {
		a := Attribute{Name: name}
		m := masks[name]
		var text, data bytes.Buffer
		redact(&text, &data, vals[name], m)
		switch {
		case clean(m):
			a.Value = compact(vals[name])
			if a.Value == nil {
				a.Value = json.RawMessage("null")
			}
			a.Text = string(a.Value)
		case text.String() == sensitiveText:
			a.Sensitive = true
			a.Text = sensitiveText
		default:
			a.Value = data.Bytes()
			a.SensitiveValues = compact(m)
			a.Text = text.String()
		}
		out = append(out, a)
	}
	return out
}

func redact(text, data *bytes.Buffer, raw, mask json.RawMessage) {
	if clean(mask) {
		v := compact(raw)
		if v == nil {
			v = json.RawMessage("null")
		}
		text.Write(v)
		data.Write(v)
		return
	}
	var maskObj, rawObj map[string]json.RawMessage
	if json.Unmarshal(mask, &maskObj) == nil && json.Unmarshal(raw, &rawObj) == nil && maskObj != nil && rawObj != nil {
		keys := slices.Sorted(maps.Keys(rawObj))
		text.WriteByte('{')
		data.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				text.WriteByte(',')
				data.WriteByte(',')
			}
			key, _ := json.Marshal(k)
			text.Write(key)
			text.WriteByte(':')
			data.Write(key)
			data.WriteByte(':')
			redact(text, data, rawObj[k], maskObj[k])
		}
		text.WriteByte('}')
		data.WriteByte('}')
		return
	}
	var maskArr, rawArr []json.RawMessage
	if json.Unmarshal(mask, &maskArr) == nil && json.Unmarshal(raw, &rawArr) == nil && maskArr != nil && rawArr != nil && len(maskArr) == len(rawArr) {
		text.WriteByte('[')
		data.WriteByte('[')
		for i := range rawArr {
			if i > 0 {
				text.WriteByte(',')
				data.WriteByte(',')
			}
			redact(text, data, rawArr[i], maskArr[i])
		}
		text.WriteByte(']')
		data.WriteByte(']')
		return
	}
	text.WriteString(sensitiveText)
	data.WriteString("null")
}

func clean(mask json.RawMessage) bool {
	if isNull(mask) {
		return true
	}
	var b bool
	if json.Unmarshal(mask, &b) == nil {
		return !b
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(mask, &obj) == nil {
		for _, m := range obj {
			if !clean(m) {
				return false
			}
		}
		return true
	}
	var arr []json.RawMessage
	if json.Unmarshal(mask, &arr) == nil {
		for _, m := range arr {
			if !clean(m) {
				return false
			}
		}
		return true
	}
	return false
}

func isNull(raw json.RawMessage) bool {
	t := bytes.TrimSpace(raw)
	return len(t) == 0 || string(t) == "null"
}

func showText(r Report) string {
	var b strings.Builder
	b.WriteString(showHeader(r))
	b.WriteByte('\n')
	if in := r.Resource; in != nil {
		for _, a := range in.Attributes {
			if !a.Sensitive && a.SensitiveValues == nil && string(a.Value) == "null" {
				continue
			}
			fmt.Fprintf(&b, "%s = %s\n", core.Line(a.Name), core.Ellipsis(core.Line(a.Text), maxValueBytes))
		}
		if in.Null > 0 {
			fmt.Fprintf(&b, "... %d null %s (--output json)\n", in.Null, core.Plural(in.Null, "attribute", "attributes"))
		}
	}
	for _, d := range r.Diagnostics {
		tfdiag.Write(&b, d, tfdiag.WriteOptions{})
	}
	return b.String()
}

func showHeader(r Report) string {
	line := r.Tool + " state show"
	var parts []string
	if in := r.Resource; in != nil {
		line += " " + in.Address
		if in.Tainted {
			line += " (tainted)"
		}
		parts = append(parts, fmt.Sprintf("%d %s", len(in.Attributes), core.Plural(len(in.Attributes), "attribute", "attributes")))
		if in.Sensitive > 0 {
			parts = append(parts, fmt.Sprintf("%d sensitive", in.Sensitive))
		}
	}
	if r.Errors > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", r.Errors, core.Plural(r.Errors, "error", "errors")))
	}
	if r.Warnings > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", r.Warnings, core.Plural(r.Warnings, "warning", "warnings")))
	}
	if len(parts) == 0 {
		return line + ": no resource"
	}
	return line + ": " + strings.Join(parts, ", ")
}

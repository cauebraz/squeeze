package tfstate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/logscan"
	"github.com/cauebraz/squeeze/internal/sanitize"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const sensitiveText = "(sensitive)"

type rawShow struct {
	FormatVersion string       `json:"format_version"`
	Resource      *rawInstance `json:"resource"`
	Diagnostics   []tfdiag.Raw `json:"diagnostics"`
}

type rawInstance struct {
	Address         string          `json:"address"`
	Mode            string          `json:"mode"`
	Type            string          `json:"type"`
	ProviderName    string          `json:"provider_name"`
	Values          json.RawMessage `json:"values"`
	SensitiveValues json.RawMessage `json:"sensitive_values"`
	Tainted         bool            `json:"tainted"`
	values          map[string]json.RawMessage
}

type instance struct {
	Address      string      `json:"address"`
	Mode         string      `json:"mode"`
	Type         string      `json:"type"`
	ProviderName string      `json:"provider_name"`
	Tainted      bool        `json:"tainted,omitempty"`
	Sensitive    int         `json:"sensitive"`
	Null         int         `json:"null"`
	Attributes   []attribute `json:"attributes"`
}

type attribute struct {
	Name            string          `json:"name"`
	Sensitive       bool            `json:"sensitive,omitempty"`
	Value           json.RawMessage `json:"value,omitempty"`
	SensitiveValues json.RawMessage `json:"sensitive_values,omitempty"`
	Text            string          `json:"-"`
}

func parseShow(input []byte) (document, error) {
	var s rawShow
	if err := json.NewDecoder(bytes.NewReader(input)).Decode(&s); err != nil {
		return document{}, fmt.Errorf("decode: %w", err)
	}
	if err := tfdiag.CheckFormatVersion(s.FormatVersion); err != nil {
		return document{}, err
	}
	if r := s.Resource; r != nil && !isNull(r.Values) {
		if err := json.Unmarshal(r.Values, &r.values); err != nil {
			return document{}, fmt.Errorf("decode values: %w", err)
		}
		if len(r.SensitiveValues) == 0 {
			return document{}, fmt.Errorf("resource %q: %w", sanitize.Clean(r.Address), errMissingSensitive)
		}
	}
	return document{Show: &s}, nil
}

func buildShow(r *report, s *rawShow, lines []logscan.FlaggedLine) {
	r.Source = sourceResource
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
	in := &instance{
		Address:      sanitize.Clean(raw.Address),
		Mode:         sanitize.Clean(raw.Mode),
		Type:         sanitize.Clean(raw.Type),
		ProviderName: sanitize.Clean(raw.ProviderName),
		Tainted:      raw.Tainted,
		Attributes:   []attribute{},
	}
	attrs := attributes(raw.values, raw.SensitiveValues)
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

type mask struct {
	raw   json.RawMessage
	clean bool
	obj   map[string]*mask
	arr   []*mask
}

var sensitiveMask = &mask{raw: json.RawMessage("true")}

func parseMask(raw json.RawMessage) *mask {
	m := &mask{raw: raw}
	if isNull(raw) {
		m.clean = true
		return m
	}
	var b bool
	if json.Unmarshal(raw, &b) == nil {
		m.clean = !b
		return m
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) == nil {
		m.obj, m.clean = make(map[string]*mask, len(obj)), true
		for k, v := range obj {
			child := parseMask(v)
			m.obj[k] = child
			m.clean = m.clean && child.clean
		}
		return m
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		m.arr, m.clean = make([]*mask, 0, len(arr)), true
		for _, v := range arr {
			child := parseMask(v)
			m.arr = append(m.arr, child)
			m.clean = m.clean && child.clean
		}
	}
	return m
}

func (m *mask) isClean() bool { return m == nil || m.clean }

func (m *mask) field(k string) *mask {
	if m == nil {
		return nil
	}
	return m.obj[k]
}

func attributes(vals map[string]json.RawMessage, raw json.RawMessage) []attribute {
	root := parseMask(raw)
	names := slices.Sorted(maps.Keys(vals))
	out := make([]attribute, 0, len(names))
	for _, name := range names {
		a := attribute{Name: name}
		var m *mask
		switch {
		case root.obj != nil:
			m = root.obj[name]
		case !root.clean:
			m = sensitiveMask
		}
		var text, data bytes.Buffer
		redact(&text, &data, vals[name], m)
		switch {
		case m.isClean():
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
			a.SensitiveValues = compact(m.raw)
			a.Text = text.String()
		}
		out = append(out, a)
	}
	return out
}

func redact(text, data *bytes.Buffer, raw json.RawMessage, m *mask) {
	if m.isClean() {
		v := compact(raw)
		if v == nil {
			v = json.RawMessage("null")
		}
		text.Write(v)
		data.Write(v)
		return
	}
	var rawObj map[string]json.RawMessage
	if m.obj != nil && json.Unmarshal(raw, &rawObj) == nil && rawObj != nil {
		keys := slices.Sorted(maps.Keys(rawObj))
		text.WriteByte('{')
		data.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				text.WriteByte(',')
				data.WriteByte(',')
			}
			key, err := json.Marshal(k)
			if err != nil {
				key = []byte(`""`)
			}
			text.Write(key)
			text.WriteByte(':')
			data.Write(key)
			data.WriteByte(':')
			redact(text, data, rawObj[k], m.field(k))
		}
		text.WriteByte('}')
		data.WriteByte('}')
		return
	}
	var rawArr []json.RawMessage
	if m.arr != nil && json.Unmarshal(raw, &rawArr) == nil && rawArr != nil && len(m.arr) == len(rawArr) {
		text.WriteByte('[')
		data.WriteByte('[')
		for i := range rawArr {
			if i > 0 {
				text.WriteByte(',')
				data.WriteByte(',')
			}
			redact(text, data, rawArr[i], m.arr[i])
		}
		text.WriteByte(']')
		data.WriteByte(']')
		return
	}
	text.WriteString(sensitiveText)
	data.WriteString("null")
}

func isNull(raw json.RawMessage) bool {
	t := bytes.TrimSpace(raw)
	return len(t) == 0 || string(t) == "null"
}

func showText(r report) string {
	var b strings.Builder
	b.WriteString(showHeader(r))
	b.WriteByte('\n')
	if in := r.Resource; in != nil {
		for _, a := range in.Attributes {
			if !a.Sensitive && a.SensitiveValues == nil && string(a.Value) == "null" {
				continue
			}
			fmt.Fprintf(&b, "%s = %s\n", sanitize.Clean(a.Name), core.Ellipsis(sanitize.Line(a.Text), maxValueBytes))
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

func showHeader(r report) string {
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
	parts = append(parts, tfdiag.CountWords(r.Errors, r.Warnings)...)
	if len(parts) == 0 {
		return line + ": no resource"
	}
	return line + ": " + strings.Join(parts, ", ")
}

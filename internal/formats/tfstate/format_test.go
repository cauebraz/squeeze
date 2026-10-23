package tfstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
	"github.com/cauebraz/squeeze/internal/tfdiag"
)

const fixtureDir = "testdata"

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixtureDir, name, "input.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestContract(t *testing.T) {
	t.Parallel()
	formattest.Contract(t, Format{}, fixtureDir)
}

func TestRealInputs(t *testing.T) {
	t.Parallel()
	formattest.Real(t, Format{}, "testdata/real")
}

func TestDetect(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input string
		want  bool
	}{
		"indented":        {"{\n  \"a\": {\n    \"sensitive\": false,\n    \"type\": \"string\",\n    \"value\": \"x\"\n  }\n}\n", true},
		"compact":         {`{"a":{"sensitive":true,"type":"string","value":"x"}}`, true},
		"escaped name":    {`{"a\"b":{"sensitive":false,"type":"bool","value":true}}`, true},
		"empty object":    {`{}`, false},
		"single value":    {`"acme-logs-prod"`, false},
		"no type":         {`{"a":{"sensitive":false,"value":"x"}}`, false},
		"sensitive word":  {`{"a":{"sensitive":"yes","type":"string"}}`, false},
		"state":           {`{"format_version":"1.0","terraform_version":"1.9.5","values":{"outputs":{},"root_module":{}}}`, true},
		"state no module": {`{"format_version":"1.0","terraform_version":"1.9.5","values":{"outputs":{"a":{"sensitive":false,"value":"x","type":"string"}}}}`, false},
		"plan prior":      {`{"format_version":"1.2","planned_values":{"root_module":{}},"prior_state":{"values":{"root_module":{}}}}`, false},
		"state in array":  {`[{"format_version":"1.0","values":{"root_module":{}}}]`, false},
		"plan":            {`{"format_version":"1.2","planned_values":{},"output_changes":{}}`, false},
		"validate":        {`{"format_version":"1.0","valid":true,"diagnostics":[]}`, false},
		"array of object": {`[{"a":{"sensitive":false,"type":"string","value":"x"}}]`, false},
		"state show":      {`{"format_version":"1.0","resource":{"address":"a.b","values":{},"sensitive_values":{}},"diagnostics":[]}`, true},
		"show error":      {"{\n  \"format_version\": \"1.0\",\n  \"resource\": {\n    \"provider_name\": \"\"\n  },\n  \"diagnostics\": []\n}", true},
		"show array":      {`{"format_version":"1.0","resource":[]}`, false},
		"show second key": {`{"format_version":"1.0","valid":true,"resource":{}}`, false},
	}
	for name, tt := range tests {
		if got := (Format{}).Detect([]byte(tt.input)); got != tt.want {
			t.Errorf("%s: Detect() = %v, want %v", name, got, tt.want)
		}
	}
	for name, input := range formattest.Inputs(t, fixtureDir) {
		if !(Format{}).Detect(input) {
			t.Errorf("fixture %s not detected", name)
		}
	}
	siblings, err := filepath.Glob("../*/testdata/*/input.*")
	if err != nil || len(siblings) == 0 {
		t.Fatalf("sibling inputs: %v, %v", siblings, err)
	}
	for _, path := range siblings {
		if strings.HasPrefix(path, "../tfstate/") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if (Format{}).Detect(core.Head(data)) {
			t.Errorf("%s: detected as tfstate", path)
		}
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input string
		err   error
		msg   string
	}{
		"truncated":         {`{"a":{"sensitive":false,`, nil, "decode"},
		"array":             {`[]`, nil, "decode"},
		"null":              {`null`, errNotObject, ""},
		"no sensitive":      {`{"a":{"type":"string","value":"x"}}`, errMissingSensitive, `output "a"`},
		"sensitive string":  {`{"a":{"sensitive":"true","type":"string","value":"x"}}`, nil, "decode"},
		"output not object": {`{"a":"x"}`, nil, "decode"},
		"state major 2":     {`{"format_version":"2.0","values":{"root_module":{}}}`, tfdiag.ErrUnsupportedVersion, "unsupported format_version"},
		"state output":      {`{"format_version":"1.0","values":{"outputs":{"a":{"value":"x"}},"root_module":{}}}`, errMissingSensitive, `output "a"`},
		"state values":      {`{"format_version":"1.0","values":[]}`, nil, "decode"},
		"show major 2":      {`{"format_version":"2.0","resource":{}}`, tfdiag.ErrUnsupportedVersion, "unsupported format_version"},
		"state no version":  {`{"format_version":"","values":{"root_module":{}}}`, tfdiag.ErrMissingVersion, ""},
		"show no mask":      {`{"format_version":"1.0","resource":{"address":"a.b","values":{"x":"s"}}}`, errMissingSensitive, `resource "a.b"`},
		"show values array": {`{"format_version":"1.0","resource":{"address":"a.b","values":[],"sensitive_values":{}}}`, nil, "decode values"},
		"show not object":   {`{"format_version":"1.0","resource":[]}`, nil, "decode"},
	}
	for name, tt := range tests {
		_, err := parse([]byte(tt.input))
		if err == nil || (tt.err != nil && !errors.Is(err, tt.err)) || !strings.Contains(err.Error(), tt.msg) {
			t.Errorf("%s: Parse() error = %v", name, err)
		}
		if _, err := squeeze([]byte(tt.input), defaultOptions()); err == nil {
			t.Errorf("%s: Squeeze() returned no error", name)
		}
	}
}

func TestSensitiveValuesNeverPrinted(t *testing.T) {
	t.Parallel()
	input := fixture(t, "outputs")
	raw, err := parse(input)
	if err != nil {
		t.Fatal(err)
	}
	var secrets []string
	for _, o := range raw.Outputs {
		if *o.Sensitive {
			secrets = append(secrets, leaves(t, o.Value)...)
		}
	}
	if len(secrets) < 3 {
		t.Fatalf("fixture holds %d sensitive values, want at least 3", len(secrets))
	}
	for _, o := range []options{defaultOptions(), {MaxOutputs: 0}} {
		r, err := squeeze(input, o)
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := core.Envelope("tfstate", len(input), r, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range secrets {
			if strings.Contains(r.Text, s) || strings.Contains(string(envelope), s) {
				t.Errorf("sensitive value %q reached the output", s)
			}
		}
	}
}

func leaves(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			for _, e := range v {
				walk(e)
			}
		case []any:
			for _, e := range v {
				walk(e)
			}
		case string:
			out = append(out, v)
		}
	}
	walk(v)
	return out
}

func TestNoForgedLines(t *testing.T) {
	t.Parallel()
	input := []byte(`{"a\nb = forged":{"sensitive":false,"type":"string","value":"x\ny\u001b[31m\r\u0085z"}}`)
	r, err := squeeze(input, defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	formattest.CheckText(t, r.Text)
	if lines := strings.Count(r.Text, "\n"); lines != 2 {
		t.Errorf("text has %d lines, want 2:\n%s", lines, r.Text)
	}
	state := []byte(`{"format_version":"1.0","terraform_version":"1.9\n5","values":{"root_module":{"resources":[{"address":"a.b\nforged.c","mode":"managed","deposed_key":"k\u001b[31m\n"}]}}}`)
	r, err = squeeze(state, defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	formattest.CheckText(t, r.Text)
	if lines := strings.Count(r.Text, "\n"); lines != 2 {
		t.Errorf("state text has %d lines, want 2:\n%s", lines, r.Text)
	}
}

func TestStateValuesNeverPrinted(t *testing.T) {
	t.Parallel()
	input := fixture(t, "state")
	var doc struct {
		Values struct {
			Outputs    map[string]rawOutput `json:"outputs"`
			RootModule json.RawMessage      `json:"root_module"`
		} `json:"values"`
	}
	if err := json.Unmarshal(input, &doc); err != nil {
		t.Fatal(err)
	}
	shown := map[string]bool{}
	var secrets []string
	for _, o := range doc.Values.Outputs {
		for _, leaf := range leaves(t, o.Value) {
			if *o.Sensitive {
				secrets = append(secrets, leaf)
			} else {
				shown[leaf] = true
			}
		}
	}
	var modules func(json.RawMessage)
	modules = func(raw json.RawMessage) {
		var m struct {
			Resources []struct {
				Values json.RawMessage `json:"values"`
			} `json:"resources"`
			ChildModules []json.RawMessage `json:"child_modules"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		for _, res := range m.Resources {
			for _, leaf := range leaves(t, res.Values) {
				if !shown[leaf] {
					secrets = append(secrets, leaf)
				}
			}
		}
		for _, child := range m.ChildModules {
			modules(child)
		}
	}
	modules(doc.Values.RootModule)
	if !slices.Contains(secrets, "hunter2-STATE") || !slices.Contains(secrets, "hunter2-STATE-OUTPUT") || !slices.Contains(secrets, "10.0.2.0/24") {
		t.Fatalf("fixture values = %v", secrets)
	}
	for _, o := range []options{defaultOptions(), {}} {
		r, err := squeeze(input, o)
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := core.Envelope("tfstate", len(input), r, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range secrets {
			if strings.Contains(r.Text, s) || strings.Contains(string(envelope), s) {
				t.Errorf("attribute or sensitive value %q reached the output", s)
			}
		}
	}
}

func TestStateShowSensitiveNeverPrinted(t *testing.T) {
	t.Parallel()
	want := map[string][]string{
		"state-show":        {"BCRYPT-SECRET-$2a$10$abcdef", "KEEPER-SECRET-77x", "RESULT-SECRET-k3j9"},
		"state-show-nested": {"LIST-SECRET-9f2", "NESTED-SECRET-4kq"},
	}
	for name, known := range want {
		input := fixture(t, name)
		var doc struct {
			Resource struct {
				Values          json.RawMessage `json:"values"`
				SensitiveValues json.RawMessage `json:"sensitive_values"`
			} `json:"resource"`
		}
		if err := json.Unmarshal(input, &doc); err != nil {
			t.Fatal(err)
		}
		var values, mask any
		if err := json.Unmarshal(doc.Resource.Values, &values); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(doc.Resource.SensitiveValues, &mask); err != nil {
			t.Fatal(err)
		}
		shown := map[string]bool{}
		var hidden []string
		var walk func(v, m any, sensitive bool)
		walk = func(v, m any, sensitive bool) {
			if b, ok := m.(bool); ok && b {
				sensitive = true
			}
			switch v := v.(type) {
			case map[string]any:
				mm, _ := m.(map[string]any)
				for k, e := range v {
					walk(e, mm[k], sensitive)
				}
			case []any:
				ma, _ := m.([]any)
				for i, e := range v {
					var em any
					if i < len(ma) {
						em = ma[i]
					}
					walk(e, em, sensitive)
				}
			case string:
				if sensitive {
					hidden = append(hidden, v)
				} else {
					shown[v] = true
				}
			}
		}
		walk(values, mask, false)
		var secrets []string
		for _, h := range hidden {
			if !shown[h] {
				secrets = append(secrets, h)
			}
		}
		for _, k := range known {
			if !slices.Contains(secrets, k) {
				t.Fatalf("%s: sensitive values %q lack %q", name, secrets, k)
			}
		}
		r, err := squeeze(input, defaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := core.Envelope("tfstate", len(input), r, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range secrets {
			if strings.Contains(r.Text, s) || strings.Contains(string(envelope), s) {
				t.Errorf("%s: sensitive value %q reached the output", name, s)
			}
		}
	}
}

func TestStateShowRedact(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		values, mask string
		want         []attribute
	}{
		"whole resource": {`{"a":"x","b":1}`, `true`, []attribute{
			{Name: "a", Sensitive: true, Text: sensitiveText},
			{Name: "b", Sensitive: true, Text: sensitiveText},
		}},
		"empty array mask":   {`{"a":"x"}`, `[]`, []attribute{{Name: "a", Value: json.RawMessage(`"x"`), Text: `"x"`}}},
		"mask type mismatch": {`{"a":["x"]}`, `{"a":{"0":true}}`, []attribute{{Name: "a", Sensitive: true, Text: sensitiveText}}},
		"length mismatch":    {`{"a":["x","y"]}`, `{"a":[true]}`, []attribute{{Name: "a", Sensitive: true, Text: sensitiveText}}},
		"unknown mask value": {`{"a":"x","b":"y"}`, `{"a":"yes","b":1}`, []attribute{
			{Name: "a", Sensitive: true, Text: sensitiveText},
			{Name: "b", Sensitive: true, Text: sensitiveText},
		}},
		"false leaves": {`{"a":{"k":[1,2]}}`, `{"a":{"k":[false,false]}}`, []attribute{{Name: "a", Value: json.RawMessage(`{"k":[1,2]}`), Text: `{"k":[1,2]}`}}},
		"nested": {`{"a":{"k":"s","l":"v"}}`, `{"a":{"k":true}}`, []attribute{
			{Name: "a", Value: json.RawMessage(`{"k":null,"l":"v"}`), SensitiveValues: json.RawMessage(`{"k":true}`), Text: `{"k":(sensitive),"l":"v"}`},
		}},
	}
	for name, tt := range tests {
		var vals map[string]json.RawMessage
		if err := json.Unmarshal([]byte(tt.values), &vals); err != nil {
			t.Fatal(err)
		}
		got := attributes(vals, json.RawMessage(tt.mask))
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("%s: attributes() (-want +got):\n%s", name, diff)
		}
	}
}

func TestStateShowBuild(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		header     string
		conditions []string
		nulls      int
	}{
		"state-show":         {"terraform state show random_password.db: 15 attributes, 3 sensitive", []string{}, 1},
		"state-show-nested":  {"terraform state show terraform_data.app (tainted): 5 attributes, 2 sensitive", []string{}, 2},
		"state-show-error":   {"terraform state show: 1 error", []string{"error", "fail"}, -1},
		"state-show-warning": {"terraform state show random_pet.name[0]: 5 attributes, 1 warning", []string{"warning"}, 2},
	}
	for name, tt := range tests {
		r, err := squeeze(fixture(t, name), defaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		header, _, _ := strings.Cut(r.Text, "\n")
		if header != tt.header {
			t.Errorf("%s: header = %q, want %q", name, header, tt.header)
		}
		if diff := cmp.Diff(tt.conditions, r.Conditions); diff != "" {
			t.Errorf("%s: conditions (-want +got):\n%s", name, diff)
		}
		rep := r.Data.(report)
		if rep.Source != "resource" || rep.State != nil || len(rep.Outputs) != 0 {
			t.Errorf("%s: report = %+v", name, rep)
		}
		if got := -1; rep.Resource != nil {
			got = rep.Resource.Null
			if got != tt.nulls {
				t.Errorf("%s: null = %d, want %d", name, got, tt.nulls)
			}
		} else if tt.nulls != -1 {
			t.Errorf("%s: no resource", name)
		}
	}
	empty, err := squeeze([]byte(`{"format_version":"1.0","resource":{"address":"a.b"},"diagnostics":[]}`), defaultOptions())
	if err != nil || empty.Text != "terraform state show a.b: 0 attributes\n" {
		t.Errorf("Squeeze(no values) = %q, %v", empty.Text, err)
	}
	none, err := squeeze([]byte(`{"format_version":"1.0","resource":null}`), defaultOptions())
	if err != nil || none.Text != "terraform state show: no resource\n" {
		t.Errorf("Squeeze(null resource) = %q, %v", none.Text, err)
	}
}

func TestStateBuild(t *testing.T) {
	t.Parallel()
	in, err := parse(fixture(t, "state"))
	if err != nil {
		t.Fatal(err)
	}
	r := build(in, defaultOptions())
	var got []string
	for _, res := range r.State.Resources {
		got = append(got, res.Address+" "+res.Mode+" "+res.DeposedKey)
	}
	want := []string{
		"aws_db_instance.main managed ",
		"aws_instance.web[0] managed ",
		"aws_instance.web[1] managed ",
		"aws_instance.web[1] managed 00000001",
		"aws_s3_bucket.logs managed ",
		"data.aws_iam_policy_document.logs data ",
		`module.vpc.aws_subnet.private["a"] managed `,
		`module.vpc.aws_subnet.private["b"] managed `,
		"module.vpc.module.nat.aws_nat_gateway.this managed ",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("resources (-want +got):\n%s", diff)
	}
	if r.Source != "state" || r.ToolVersion != "1.9.5" || r.State.Managed != 7 || r.State.DataSources != 1 || r.State.Tainted != 1 || r.State.Deposed != 1 || r.Total != 3 || r.Sensitive != 1 {
		t.Errorf("report = %+v", r)
	}
	limited := build(in, options{MaxResources: 2, MaxOutputs: 1})
	if s := limited.State; len(s.Resources) != 2 || s.ResourcesOmitted != 7 || len(limited.Outputs) != 1 || limited.Omitted != 2 || s.Managed != 7 {
		t.Errorf("limited = %+v", limited)
	}
}

func TestStateHeader(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input string
		want  string
	}{
		"empty":        {`{"format_version":"1.0"}`, "terraform state: 0 resources\n"},
		"outputs only": {`{"format_version":"1.0","terraform_version":"1.9.5","values":{"outputs":{"a":{"sensitive":false,"value":1,"type":"number"}},"root_module":{}}}`, "terraform 1.9.5 state: 0 resources; 1 output\noutputs:\na = 1\n"},
		"one data":     {`{"format_version":"1.0","values":{"root_module":{"resources":[{"address":"data.x.y","mode":"data"}]}}}`, "terraform state: 0 resources, 1 data source\ndata.x.y\n"},
	}
	for name, tt := range tests {
		r, err := squeeze([]byte(tt.input), defaultOptions())
		if err != nil || r.Text != tt.want {
			t.Errorf("%s: Squeeze() = %q, %v, want %q", name, r.Text, err, tt.want)
		}
	}
}

func TestBuild(t *testing.T) {
	t.Parallel()
	raw, err := parse(fixture(t, "outputs"))
	if err != nil {
		t.Fatal(err)
	}
	r := build(raw, defaultOptions())
	var names []string
	for _, o := range r.Outputs {
		names = append(names, o.Name)
		if o.Sensitive != (o.Value == nil) {
			t.Errorf("%s: sensitive = %v, value = %s", o.Name, o.Sensitive, o.Value)
		}
	}
	want := []string{"api_url", "app", "banner", "db", "db_password", "enabled", "endpoints", "private_subnet_ids", "replicas", "tags", "unset"}
	if diff := cmp.Diff(want, names); diff != "" {
		t.Errorf("order (-want +got):\n%s", diff)
	}
	if r.Total != 11 || r.Sensitive != 2 || r.Omitted != 0 {
		t.Errorf("report = %+v", r)
	}
	limited := build(raw, options{MaxOutputs: 4})
	if len(limited.Outputs) != 4 || limited.Omitted != 7 || limited.Total != 11 || limited.Sensitive != 2 {
		t.Errorf("limited = %+v", limited)
	}
}

func TestLongValueKeptInData(t *testing.T) {
	t.Parallel()
	r, err := squeeze(fixture(t, "outputs"), defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	var full string
	for _, o := range r.Data.(report).Outputs {
		if o.Name == "private_subnet_ids" {
			full = string(o.Value)
		}
	}
	if len(full) <= maxValueBytes || !strings.HasSuffix(full, `"subnet-020a1b2c3d4e5f6"]`) {
		t.Fatalf("data value = %s", full)
	}
	if !strings.Contains(r.Text, "private_subnet_ids = "+full[:maxValueBytes-3]+"...\n") {
		t.Errorf("text:\n%s", r.Text)
	}
}

func TestEmptyAndMissingValue(t *testing.T) {
	t.Parallel()
	r, err := squeeze([]byte("{}\n"), defaultOptions())
	if err != nil || r.Text != "terraform output: 0 outputs\n" {
		t.Errorf("Squeeze({}) = %q, %v", r.Text, err)
	}
	r, err = squeeze([]byte(`{"a":{"sensitive":false}}`), defaultOptions())
	if err != nil || r.Text != "terraform output: 1 output\na = null\n" {
		t.Errorf("Squeeze(no value) = %q, %v", r.Text, err)
	}
	if diff := cmp.Diff([]string{}, r.Conditions); diff != "" {
		t.Errorf("conditions (-want +got):\n%s", diff)
	}
}

func TestConfigureFlags(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		args []string
		msg  string
	}{{[]string{"-max-outputs", "x"}, "not an integer"}, {[]string{"-max-outputs", "-1"}, "must be 0 or more"}} {
		fs := flag.NewFlagSet("tfstate", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		(Format{}).Configure(fs)
		if err := fs.Parse(tt.args); err == nil || !strings.Contains(err.Error(), tt.msg) {
			t.Errorf("Parse(%v) error = %v", tt.args, err)
		}
	}
	var out bytes.Buffer
	fs := flag.NewFlagSet("tfstate", flag.ContinueOnError)
	fs.SetOutput(&out)
	(Format{}).Configure(fs)
	fs.PrintDefaults()
	if !strings.Contains(out.String(), "(default 50)") {
		t.Errorf("usage:\n%s", out.String())
	}
}

func TestTrailingLines(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input string
		text  string
		conds []string
	}{
		"state": {
			input: "{\"format_version\":\"1.0\",\"terraform_version\":\"1.9.5\",\"values\":{\"root_module\":{}}}\nWarning: deprecated\nplain\n",
			text:  "terraform 1.9.5 state: 0 resources; 1 warning\nwarning: deprecated\n",
			conds: []string{"warning"},
		},
		"show": {
			input: "{\"format_version\":\"1.0\",\"resource\":null,\"diagnostics\":[{\"severity\":\"warning\",\"summary\":\"w\"}]}\nError: lock held\n",
			text:  "terraform state show: 1 error, 1 warning\nerror: lock held\nwarning: w\n",
			conds: []string{"error", "fail", "warning"},
		},
		"plain lines": {
			input: "{}\ndone\n",
			text:  "terraform output: 0 outputs\n",
			conds: []string{},
		},
	}
	for name, tc := range tests {
		r, err := squeeze([]byte(tc.input), defaultOptions())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if diff := cmp.Diff(tc.text, r.Text); diff != "" {
			t.Errorf("%s: text (-want +got):\n%s", name, diff)
		}
		if diff := cmp.Diff(tc.conds, r.Conditions); diff != "" {
			t.Errorf("%s: conditions (-want +got):\n%s", name, diff)
		}
	}
}

const canary = "s3cret"

func cleanMask(mask any) bool {
	switch m := mask.(type) {
	case nil:
		return true
	case bool:
		return !m
	case map[string]any:
		for _, v := range m {
			if !cleanMask(v) {
				return false
			}
		}
		return true
	case []any:
		for _, v := range m {
			if !cleanMask(v) {
				return false
			}
		}
		return true
	}
	return false
}

func canaries(v any) int {
	n := 0
	switch t := v.(type) {
	case string:
		if strings.Contains(t, canary) {
			n++
		}
	case map[string]any:
		for k, e := range t {
			n += canaries(k) + canaries(e)
		}
	case []any:
		for _, e := range t {
			n += canaries(e)
		}
	}
	return n
}

func hiddenCanaries(v, mask any) int {
	if cleanMask(mask) {
		return 0
	}
	if vm, ok := v.(map[string]any); ok {
		if mm, ok := mask.(map[string]any); ok {
			n := 0
			for k, e := range vm {
				n += hiddenCanaries(e, mm[k])
			}
			return n
		}
	}
	if vl, ok := v.([]any); ok {
		if ml, ok := mask.([]any); ok && len(ml) == len(vl) {
			n := 0
			for i, e := range vl {
				n += hiddenCanaries(e, ml[i])
			}
			return n
		}
	}
	return canaries(v)
}

func onlyHiddenCanaries(input []byte) bool {
	var doc any
	rest, err := core.DecodeJSON(input, &doc)
	if err != nil || bytes.Contains(rest, []byte(canary)) {
		return false
	}
	top, _ := doc.(map[string]any)
	res, _ := top["resource"].(map[string]any)
	all := canaries(doc)
	return all > 0 && all == hiddenCanaries(res["values"], res["sensitive_values"])
}

func TestOnlyHiddenCanaries(t *testing.T) {
	t.Parallel()
	tests := map[string]bool{
		`{"resource":{"values":{"p":"s3cret"},"sensitive_values":{"p":true}}}`:                true,
		`{"resource":{"values":{"p":"s3cret"},"sensitive_values":{"p":1}}}`:                   true,
		`{"resource":{"values":{"l":["a","s3cret"]},"sensitive_values":{"l":[true]}}}`:        true,
		`{"resource":{"values":{"l":["a","s3cret"]},"sensitive_values":{"l":[false,false]}}}`: false,
		`{"resource":{"values":{"p":"s3cret"},"sensitive_values":{}}}`:                        false,
		`{"resource":{"values":{"s3cret":"x"},"sensitive_values":{"s3cret":true}}}`:           false,
		`{"resource":{"values":{"p":"s3cret"},"sensitive_values":"x"}}`:                       true,
		`{"resource":{"values":{"p":"x"},"sensitive_values":{"p":true}}}`:                     false,
		"{\"resource\":{\"values\":{\"p\":\"s3cret\"},\"sensitive_values\":true}}\ns3cret":    false,
		`{`: false,
	}
	for input, want := range tests {
		if got := onlyHiddenCanaries([]byte(input)); got != want {
			t.Errorf("onlyHiddenCanaries(%s) = %v, want %v", input, got, want)
		}
	}
}

func TestMissingSensitiveNamesTheFirstOutput(t *testing.T) {
	t.Parallel()
	input := []byte(`{"d":{"type":"string","value":"x"},"b":{"type":"string","value":"x"},"c":{"type":"string","value":"x"},"a":{"sensitive":false,"type":"string","value":"x"}}`)
	for range 20 {
		_, err := squeeze(input, defaultOptions())
		if err == nil || err.Error() != `output "b": missing sensitive` {
			t.Fatalf("error = %v", err)
		}
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(`{"a\n":{"sensitive":true,"type":"string","value":"secret-value"},"b":{"sensitive":false,"type":"string","value":"\u001b[31m"}}`))
	f.Add([]byte(`{"format_version":"1.0","resource":{"address":"a.b","values":{"p":"secret-value","l":["x","secret-value"]},"sensitive_values":{"p":true,"l":[false,true]}},"diagnostics":[{"severity":"warning","summary":"w"}]}`))
	f.Add([]byte("{\"format_version\":\"1.0\",\"values\":{\"root_module\":{}}}\nError: x\nWarning: y\n"))
	f.Add([]byte("{\"format_version\":\"1.0\",\"resource\":null,\"diagnostics\":[]}\nError: x\n"))
	f.Add([]byte(`{"format_version":"1.0","resource":{"address":"a.b","values":{"p":"s3cret","q":{"k":"s3cret","n":"x"},"l":["x","s3cret"]},"sensitive_values":{"p":1,"q":{"k":true},"l":[false,true]}},"diagnostics":[]}`))
	f.Fuzz(func(t *testing.T, input []byte) {
		formattest.CheckStable(t, func() (core.Result, error) { return squeeze(input, defaultOptions()) })
		for _, o := range []options{defaultOptions(), {MaxOutputs: 1, MaxResources: 1}} {
			r, err := squeeze(input, o)
			if err != nil {
				return
			}
			formattest.CheckText(t, r.Text)
			envelope, err := core.Envelope("tfstate", len(input), r, nil)
			if err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
			rep := r.Data.(report)
			if in, err := parse(input); err == nil {
				for _, l := range in.Trailing {
					if !strings.Contains(r.Text, l.Text) {
						t.Fatalf("line after the JSON dropped: %q\n%s", l.Text, r.Text)
					}
					if l.Error && rep.Errors == 0 {
						t.Fatalf("error line %q counts no error", l.Text)
					}
				}
			}
			if o.MaxOutputs > 0 && len(rep.Outputs) > o.MaxOutputs {
				t.Fatalf("%d outputs over the limit %d", len(rep.Outputs), o.MaxOutputs)
			}
			if rep.Source == "resource" && onlyHiddenCanaries(input) && (strings.Contains(r.Text, canary) || bytes.Contains(envelope, []byte(canary))) {
				t.Fatalf("sensitive value %q reached the output:\n%s\n%s", canary, r.Text, envelope)
			}
			if rep.Source == "resource" {
				attrs := []attribute{}
				if rep.Resource != nil {
					attrs = rep.Resource.Attributes
				}
				for _, a := range attrs {
					if a.Sensitive && a.Value != nil {
						t.Fatalf("sensitive attribute %q keeps its value in data: %s", a.Name, envelope)
					}
					if a.Sensitive && strings.Contains(r.Text, a.Name+" = ") && !strings.Contains(r.Text, a.Name+" = "+sensitiveText+"\n") {
						t.Fatalf("sensitive attribute %q prints a value:\n%s", a.Name, r.Text)
					}
				}
				if got, limit := strings.Count(r.Text, "\n"), 2+len(attrs)+12*len(rep.Diagnostics); got > limit {
					t.Fatalf("text has %d lines, over %d", got, limit)
				}
				continue
			}
			for _, out := range rep.Outputs {
				if out.Sensitive && out.Value != nil {
					t.Fatalf("sensitive output %q keeps its value in data: %s", out.Name, envelope)
				}
			}
			listed := len(rep.Outputs) + 12*len(rep.Diagnostics)
			if rep.State != nil {
				listed += len(rep.State.Resources) + 2
				if o.MaxResources > 0 && len(rep.State.Resources) > o.MaxResources {
					t.Fatalf("%d resources over the limit %d", len(rep.State.Resources), o.MaxResources)
				}
			}
			if got := strings.Count(r.Text, "\n"); got > listed+2 {
				t.Fatalf("text has %d lines for %d items", got, listed)
			}
		}
	})
}

func TestAddressesAreTrimmed(t *testing.T) {
	t.Parallel()
	state := []byte(`{"format_version":"1.0","values":{"root_module":{"resources":[{"address":"\t aws_s3_bucket.a \t","mode":"managed"}]}}}`)
	r, err := squeeze(state, defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if want := "\naws_s3_bucket.a\n"; !strings.Contains(r.Text, want) {
		t.Errorf("state text lacks %q:\n%s", want, r.Text)
	}
	show := []byte(`{"format_version":"1.0","resource":{"address":"\taws_s3_bucket.a","mode":"managed","type":"aws_s3_bucket","values":{"id":"x"},"sensitive_values":{}}}`)
	r, err = squeeze(show, defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if want := "terraform state show aws_s3_bucket.a:"; !strings.HasPrefix(r.Text, want) {
		t.Errorf("show text does not start with %q:\n%s", want, r.Text)
	}
	missing := []byte(`{"format_version":"1.0","resource":{"address":"\taws_s3_bucket.a","values":{"id":"x"}}}`)
	if _, err := squeeze(missing, defaultOptions()); err == nil || !strings.Contains(err.Error(), `"aws_s3_bucket.a"`) {
		t.Errorf("Squeeze() error = %v, want the trimmed address", err)
	}
}

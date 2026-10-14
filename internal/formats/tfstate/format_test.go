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

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
	"github.com/google/go-cmp/cmp"
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
		"null":              {`null`, ErrNotObject, ""},
		"no sensitive":      {`{"a":{"type":"string","value":"x"}}`, ErrMissingSensitive, `output "a"`},
		"sensitive string":  {`{"a":{"sensitive":"true","type":"string","value":"x"}}`, nil, "decode"},
		"output not object": {`{"a":"x"}`, nil, "decode"},
		"state major 2":     {`{"format_version":"2.0","values":{"root_module":{}}}`, nil, "unsupported format_version"},
		"state output":      {`{"format_version":"1.0","values":{"outputs":{"a":{"value":"x"}},"root_module":{}}}`, ErrMissingSensitive, `output "a"`},
		"state values":      {`{"format_version":"1.0","values":[]}`, nil, "decode"},
	}
	for name, tt := range tests {
		_, err := Parse([]byte(tt.input))
		if err == nil || (tt.err != nil && !errors.Is(err, tt.err)) || !strings.Contains(err.Error(), tt.msg) {
			t.Errorf("%s: Parse() error = %v", name, err)
		}
		if _, err := Squeeze([]byte(tt.input), DefaultOptions()); err == nil {
			t.Errorf("%s: Squeeze() returned no error", name)
		}
	}
}

func TestSensitiveValuesNeverPrinted(t *testing.T) {
	t.Parallel()
	input := fixture(t, "outputs")
	raw, err := Parse(input)
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
	for _, o := range []Options{DefaultOptions(), {MaxOutputs: 0}} {
		r, err := Squeeze(input, o)
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
	r, err := Squeeze(input, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	formattest.CheckText(t, r.Text)
	if lines := strings.Count(r.Text, "\n"); lines != 2 {
		t.Errorf("text has %d lines, want 2:\n%s", lines, r.Text)
	}
	state := []byte(`{"format_version":"1.0","terraform_version":"1.9\n5","values":{"root_module":{"resources":[{"address":"a.b\nforged.c","mode":"managed","deposed_key":"k\u001b[31m\n"}]}}}`)
	r, err = Squeeze(state, DefaultOptions())
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
			Outputs    map[string]RawOutput `json:"outputs"`
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
	for _, o := range []Options{DefaultOptions(), {}} {
		r, err := Squeeze(input, o)
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

func TestStateBuild(t *testing.T) {
	t.Parallel()
	in, err := Parse(fixture(t, "state"))
	if err != nil {
		t.Fatal(err)
	}
	r := Build(in, DefaultOptions())
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
	limited := Build(in, Options{MaxResources: 2, MaxOutputs: 1})
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
		r, err := Squeeze([]byte(tt.input), DefaultOptions())
		if err != nil || r.Text != tt.want {
			t.Errorf("%s: Squeeze() = %q, %v, want %q", name, r.Text, err, tt.want)
		}
	}
}

func TestBuild(t *testing.T) {
	t.Parallel()
	raw, err := Parse(fixture(t, "outputs"))
	if err != nil {
		t.Fatal(err)
	}
	r := Build(raw, DefaultOptions())
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
	limited := Build(raw, Options{MaxOutputs: 4})
	if len(limited.Outputs) != 4 || limited.Omitted != 7 || limited.Total != 11 || limited.Sensitive != 2 {
		t.Errorf("limited = %+v", limited)
	}
}

func TestLongValueKeptInData(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "outputs"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	var full string
	for _, o := range r.Data.(Report).Outputs {
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
	r, err := Squeeze([]byte("{}\n"), DefaultOptions())
	if err != nil || r.Text != "terraform output: 0 outputs\n" {
		t.Errorf("Squeeze({}) = %q, %v", r.Text, err)
	}
	r, err = Squeeze([]byte(`{"a":{"sensitive":false}}`), DefaultOptions())
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

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(`{"a\n":{"sensitive":true,"type":"string","value":"secret-value"},"b":{"sensitive":false,"type":"string","value":"\u001b[31m"}}`))
	f.Fuzz(func(t *testing.T, input []byte) {
		for _, o := range []Options{DefaultOptions(), {MaxOutputs: 1, MaxResources: 1}} {
			r, err := Squeeze(input, o)
			if err != nil {
				return
			}
			formattest.CheckText(t, r.Text)
			envelope, err := core.Envelope("tfstate", len(input), r, nil)
			if err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
			report := r.Data.(Report)
			if o.MaxOutputs > 0 && len(report.Outputs) > o.MaxOutputs {
				t.Fatalf("%d outputs over the limit %d", len(report.Outputs), o.MaxOutputs)
			}
			for _, out := range report.Outputs {
				if out.Sensitive && out.Value != nil {
					t.Fatalf("sensitive output %q keeps its value in data: %s", out.Name, envelope)
				}
			}
			listed := len(report.Outputs)
			if report.State != nil {
				listed += len(report.State.Resources) + 2
				if o.MaxResources > 0 && len(report.State.Resources) > o.MaxResources {
					t.Fatalf("%d resources over the limit %d", len(report.State.Resources), o.MaxResources)
				}
			}
			if got := strings.Count(r.Text, "\n"); got > listed+2 {
				t.Fatalf("text has %d lines for %d items", got, listed)
			}
		}
	})
}

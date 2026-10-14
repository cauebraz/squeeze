package tfstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
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
		"state":           {`{"format_version":"1.0","terraform_version":"1.9.5","values":{"outputs":{"a":{"sensitive":false,"value":"x","type":"string"}}}}`, false},
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
	for _, o := range raw {
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
		for _, o := range []Options{DefaultOptions(), {MaxOutputs: 1}} {
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
			if got := strings.Count(r.Text, "\n"); got > len(report.Outputs)+2 {
				t.Fatalf("text has %d lines for %d outputs", got, len(report.Outputs))
			}
		}
	})
}

package tfvalidate

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
	"github.com/cauebraz/squeeze/internal/sanitize"
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
		"validate":       {`{"format_version":"1.0","valid":true,"error_count":0,"warning_count":0,"diagnostics":[]}`, true},
		"leading space":  {"\n {\"format_version\":\"1.0\",\"valid\":false,\"diagnostics\":[]}", true},
		"plan":           {`{"format_version":"1.2","valid":true,"diagnostics":[],"planned_values":{}}`, false},
		"plan changes":   {`{"format_version":"1.2","valid":true,"diagnostics":[],"resource_changes":[]}`, false},
		"no diagnostics": {`{"format_version":"1.0","valid":true}`, false},
		"stream line":    {`{"@level":"info","type":"version"}`, false},
		"array":          {`[{"format_version":"1.0","valid":true,"diagnostics":[]}]`, false},
		"empty":          {"", false},
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
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input string
		err   error
		msg   string
	}{
		"truncated":       {`{"format_version":"1.0",`, nil, "decode"},
		"no version":      {`{"valid":true,"diagnostics":[]}`, tfdiag.ErrMissingVersion, ""},
		"major 2":         {`{"format_version":"2.0","valid":true,"diagnostics":[]}`, nil, "unsupported format_version"},
		"no valid":        {`{"format_version":"1.0","diagnostics":[]}`, errMissingValid, ""},
		"bad diagnostics": {`{"format_version":"1.0","valid":true,"diagnostics":{}}`, nil, "decode"},
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

func TestBuild(t *testing.T) {
	t.Parallel()
	v, err := parse(fixture(t, "errors-and-warnings"))
	if err != nil {
		t.Fatal(err)
	}
	r := build(v, defaultOptions())
	var got []string
	for _, d := range r.Diagnostics {
		got = append(got, d.Severity+" "+d.File)
	}
	want := []string{"error main.tf", "error modules/vpc/main.tf", "warning main.tf", "warning storage.tf"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("order (-want +got):\n%s", diff)
	}
	if r.Valid || r.Errors != 2 || r.Warnings != 2 || r.Omitted != 0 {
		t.Errorf("report = %+v", r)
	}
	limited := build(v, options{MaxDiagnostics: 1})
	if len(limited.Diagnostics) != 1 || limited.Omitted != 3 || limited.Errors != 2 {
		t.Errorf("limited = %+v", limited)
	}
}

func TestHeader(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		r    report
		want string
	}{
		"valid":            {report{Tool: "terraform", Valid: true}, "terraform validate: valid"},
		"valid warnings":   {report{Tool: "terraform", Valid: true, Warnings: 2}, "terraform validate: valid, 2 warnings"},
		"invalid":          {report{Tool: "terraform", Errors: 1}, "terraform validate: invalid, 1 error"},
		"invalid both":     {report{Tool: "terraform", Errors: 2, Warnings: 1}, "terraform validate: invalid, 2 errors, 1 warning"},
		"invalid no count": {report{Tool: "terraform"}, "terraform validate: invalid, 0 errors"},
	}
	for name, tt := range tests {
		if got := header(tt.r); got != tt.want {
			t.Errorf("%s: header() = %q, want %q", name, got, tt.want)
		}
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"valid":               {},
		"duplicates":          {"warning"},
		"one-error":           {"error", "fail", "invalid"},
		"errors-and-warnings": {"error", "fail", "invalid", "warning"},
	}
	for name, want := range tests {
		r, err := squeeze(fixture(t, name), defaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, r.Conditions); diff != "" {
			t.Errorf("%s: conditions (-want +got):\n%s", name, diff)
		}
	}
}

func TestDuplicatesAreFolded(t *testing.T) {
	t.Parallel()
	r, err := squeeze(fixture(t, "duplicates"), defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Text, "(storage.tf:7:3) (x3)\n") || strings.Count(r.Text, "Argument is deprecated") != 2 {
		t.Errorf("text:\n%s", r.Text)
	}
}

func TestCountsCoverDiagnostics(t *testing.T) {
	t.Parallel()
	input := []byte(`{"format_version":"1.0","valid":false,"error_count":0,"warning_count":0,"diagnostics":[` +
		`{"severity":"error","summary":"a"},{"severity":"error","summary":"b"},{"severity":"warning","summary":"c"}]}`)
	r, err := squeeze(input, options{MaxDiagnostics: 1})
	if err != nil {
		t.Fatal(err)
	}
	rep := r.Data.(report)
	if rep.Errors != 2 || rep.Warnings != 1 {
		t.Errorf("errors = %d, warnings = %d, want 2 and 1", rep.Errors, rep.Warnings)
	}
	if diff := cmp.Diff([]string{"error", "fail", "invalid", "warning"}, r.Conditions); diff != "" {
		t.Errorf("conditions (-want +got):\n%s", diff)
	}
	if !strings.HasPrefix(r.Text, "terraform validate: invalid, 2 errors, 1 warning\n") {
		t.Errorf("text:\n%s", r.Text)
	}
	more := []byte(`{"format_version":"1.0","valid":false,"error_count":5,"warning_count":0,"diagnostics":[{"severity":"error","summary":"a"}]}`)
	r, err = squeeze(more, defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if rep := r.Data.(report); rep.Errors != 5 {
		t.Errorf("errors = %d, want Terraform's count 5", rep.Errors)
	}
}

func TestTrailingLines(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input string
		text  string
		conds []string
	}{
		"warning keeps the run valid": {
			input: "{\"format_version\":\"1.0\",\"valid\":true,\"diagnostics\":[]}\nWarning: deprecated flag\nplain\n",
			text:  "terraform validate: valid, 1 warning\nwarning: deprecated flag\n",
			conds: []string{"warning"},
		},
		"plain lines are dropped": {
			input: "{\"format_version\":\"1.0\",\"valid\":true,\"diagnostics\":[]}\ndone\n",
			text:  "terraform validate: valid\n",
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

func TestConfigureFlags(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		args []string
		msg  string
	}{{[]string{"-max-diagnostics", "x"}, "not an integer"}, {[]string{"-max-diagnostics", "-1"}, "must be 0 or more"}} {
		fs := flag.NewFlagSet("tfvalidate", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		(Format{}).Configure(fs)
		if err := fs.Parse(tt.args); err == nil || !strings.Contains(err.Error(), tt.msg) {
			t.Errorf("Parse(%v) error = %v", tt.args, err)
		}
	}
	var out bytes.Buffer
	fs := flag.NewFlagSet("tfvalidate", flag.ContinueOnError)
	fs.SetOutput(&out)
	(Format{}).Configure(fs)
	fs.PrintDefaults()
	if !strings.Contains(out.String(), "(default 20)") || !strings.Contains(out.String(), "-no-detail") {
		t.Errorf("usage:\n%s", out.String())
	}
}

func TestDataPayload(t *testing.T) {
	t.Parallel()
	r, err := squeeze(fixture(t, "valid"), defaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	out, err := core.Envelope("tfvalidate", 10, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Data struct {
			Diagnostics []any `json:"diagnostics"`
			Valid       bool  `json:"valid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &doc); err != nil || doc.Data.Diagnostics == nil || !doc.Data.Valid {
		t.Errorf("envelope = %s, err = %v", out, err)
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(`{"format_version":"1.0","valid":false,"diagnostics":[{"severity":"error","summary":"\u001b[31mx","snippet":{"code":"a\nb","highlight_start_offset":-4}}]}`))
	f.Add([]byte("{\"format_version\":\"1.0\",\"valid\":true,\"diagnostics\":[]}\nError: x\nWarning: y\n"))
	f.Fuzz(func(t *testing.T, input []byte) {
		formattest.CheckStable(t, func() (core.Result, error) { return squeeze(input, defaultOptions()) })
		for _, o := range []options{defaultOptions(), {MaxDiagnostics: 1, NoDetail: true}} {
			r, err := squeeze(input, o)
			if err != nil {
				return
			}
			formattest.CheckText(t, r.Text)
			if _, err := core.Envelope("tfvalidate", len(input), r, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
			rep := r.Data.(report)
			if v, err := parse(input); err == nil {
				raw := 0
				for _, d := range v.Diagnostics {
					if sanitize.Clean(d.Severity) != "warning" {
						raw++
					}
				}
				if rep.Errors < raw || (raw > 0 && !slices.Contains(r.Conditions, "error")) {
					t.Fatalf("%d error diagnostics in the input, %d counted, conditions %v", raw, rep.Errors, r.Conditions)
				}
				for _, l := range v.Trailing {
					if l.Error && (rep.Errors == 0 || rep.Valid) {
						t.Fatalf("error line %q after the JSON left errors=%d valid=%v", l.Text, rep.Errors, rep.Valid)
					}
				}
			}
			if o.MaxDiagnostics > 0 && len(rep.Diagnostics) > o.MaxDiagnostics {
				t.Fatalf("%d diagnostics over the limit %d", len(rep.Diagnostics), o.MaxDiagnostics)
			}
		}
	})
}

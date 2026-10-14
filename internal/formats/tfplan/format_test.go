package tfplan

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
)

func TestContract(t *testing.T) {
	t.Parallel()
	formattest.Contract(t, Format{}, fixtureDir)
}

func TestRealInputs(t *testing.T) {
	t.Parallel()
	formattest.Real(t, Format{}, "testdata/real")
	for name, input := range formattest.RealInputs(t, Format{}, "testdata/real") {
		r, err := Squeeze(input, Options{IncludeNoop: true, ListDrift: true, MaxAttrs: 0, MaxResources: 0})
		if err != nil {
			continue
		}
		envelope, err := core.Envelope("tfplan", len(input), r, nil)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		assertNoLeak(t, name, input, r.Text, string(envelope))
	}
}

func TestFormatName(t *testing.T) {
	t.Parallel()
	if got := (Format{}).Name(); got != "tfplan" {
		t.Errorf("Name() = %q, want tfplan", got)
	}
}

func TestConfigureRejectsBadCounts(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		args []string
		msg  string
	}{
		"not a number":       {[]string{"-max-attrs", "x"}, "not an integer"},
		"negative attrs":     {[]string{"-max-attrs", "-1"}, "must be 0 or more"},
		"negative resources": {[]string{"-max-resources", "-5"}, "must be 0 or more"},
	}
	for name, tt := range tests {
		fs := flag.NewFlagSet("tfplan", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		(Format{}).Configure(fs)
		if err := fs.Parse(tt.args); err == nil || !strings.Contains(err.Error(), tt.msg) {
			t.Errorf("%s: Parse(%v) error = %v, want %q", name, tt.args, err, tt.msg)
		}
	}
}

func TestConfigureSetsOptions(t *testing.T) {
	t.Parallel()
	input := fixture(t, "update-nested")
	plain, err := formattest.Run(t, Format{}, input, "-max-attrs", "1")
	if err != nil {
		t.Fatal(err)
	}
	unlimited, err := formattest.Run(t, Format{}, input, "-max-attrs", "0", "-max-resources", "0")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plain.Text, "more attributes (--max-attrs 0)") || strings.Contains(unlimited.Text, "more attributes") {
		t.Errorf("-max-attrs 0 did not lift the limit:\n%s", unlimited.Text)
	}
}

func TestConfigureUsageShowsDefaults(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	fs := flag.NewFlagSet("tfplan", flag.ContinueOnError)
	fs.SetOutput(&out)
	(Format{}).Configure(fs)
	fs.PrintDefaults()
	for _, s := range []string{"-max-attrs", "(default 8)", "-max-resources", "(default 200)", "-all", "-no-values", "-drift"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("usage lacks %q:\n%s", s, out.String())
		}
	}
}

func TestDataPayload(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "noop-only"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(r.Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"changes":[]`, `"drift":[]`, `"deferred":[]`, `"actions":[]`, `"outputs":[]`, `"tool":"terraform"`} {
		if !strings.Contains(string(out), key) {
			t.Errorf("data lacks %s: %s", key, out)
		}
	}
	created, err := Squeeze(fixture(t, "create-only"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	out, _ = json.Marshal(created.Data)
	for _, key := range []string{"previous_address", "deposed", "importing", "replaced_by", "attrs_omitted", "irrelevant"} {
		if strings.Contains(string(out), `"`+key+`"`) {
			t.Errorf("data contains empty %s: %s", key, out)
		}
	}
	nested, err := Squeeze(fixture(t, "update-nested"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	out, _ = json.Marshal(nested.Data)
	for _, want := range []string{`"before":"t3.small","after":"t3.medium"`, `"before":false,"after":true`, `"kind":"add"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("data lacks typed value %s: %s", want, out)
		}
	}
}

func TestDataPayloadInvalidEnums(t *testing.T) {
	t.Parallel()
	if _, err := json.Marshal(Report{Changes: []ResourceChange{{Address: "a", Action: Action(99)}}}); err == nil {
		t.Error("Marshal() with an invalid action returned no error")
	}
	if _, err := json.Marshal(AttrChange{Kind: AttrKind(9)}); err == nil {
		t.Error("Marshal() with an invalid kind returned no error")
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"noop-only":      {},
		"create-only":    {"change"},
		"replace-forced": {"change", "destroy", "replace"},
		"drift":          {"change", "drift"},
		"deferred":       {"change", "deferred", "fail", "incomplete"},
		"errored":        {"change", "error", "fail", "incomplete"},
		"forget-replace": {"change", "forget", "replace"},
		"checks":         {"change", "check", "warning"},
		"import":         {"change", "import"},
		"moved":          {"change", "forget", "move"},
	}
	for name, want := range tests {
		r, err := Squeeze(fixture(t, name), DefaultOptions())
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if strings.Join(r.Conditions, ",") != strings.Join(want, ",") {
			t.Errorf("%s: Conditions = %v, want %v", name, r.Conditions, want)
		}
	}
	seen := map[string]bool{}
	for _, want := range tests {
		for _, c := range want {
			seen[c] = true
		}
	}
	for _, c := range (Format{}).Conditions() {
		if !seen[c] {
			t.Errorf("declared condition %q has no fixture that produces it", c)
		}
	}
}

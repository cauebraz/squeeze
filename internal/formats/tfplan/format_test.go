package tfplan

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/formattest"
)

func TestContract(t *testing.T) {
	t.Parallel()
	formattest.Contract(t, Format{}, fixtureDir)
}

func TestRealInputs(t *testing.T) {
	t.Parallel()
	formattest.Real(t, Format{}, "testdata/real")
}

func TestFormatName(t *testing.T) {
	t.Parallel()
	if got := (Format{}).Name(); got != "tfplan" {
		t.Errorf("Name() = %q, want tfplan", got)
	}
}

func TestConfigureFlags(t *testing.T) {
	t.Parallel()
	input := fixture(t, "update-nested.json")
	tests := []struct {
		args     []string
		contains []string
		absent   []string
	}{
		{nil, []string{"  ~ instance_type: \"t3.small\" -> \"t3.medium\"\n"}, nil},
		{[]string{"-max-attrs", "1"}, []string{"  ... 2 more attributes (--max-attrs)\n"}, nil},
		{[]string{"-max-resources", "1"}, []string{"... 1 more resources (--max-resources)\n"}, []string{"module.vpc"}},
		{[]string{"-no-values"}, []string{"  ~ instance_type\n"}, []string{"t3.medium"}},
		{[]string{"-max-attrs", "0", "-max-resources", "0"}, []string{"tags_all.Name"}, []string{"more attributes", "more resources"}},
	}
	for _, tt := range tests {
		r, err := formattest.Run(t, Format{}, input, tt.args...)
		if err != nil {
			t.Fatalf("%v: %v", tt.args, err)
		}
		for _, s := range tt.contains {
			if !strings.Contains(r.Text, s) {
				t.Errorf("%v: text lacks %q:\n%s", tt.args, s, r.Text)
			}
		}
		for _, s := range tt.absent {
			if strings.Contains(r.Text, s) {
				t.Errorf("%v: text contains %q", tt.args, s)
			}
		}
	}
	all, _ := formattest.Run(t, Format{}, fixture(t, "noop-only.json"), "-all")
	if !strings.Contains(all.Text, "= aws_iam_role.app\n") {
		t.Errorf("-all: %s", all.Text)
	}
	drift, _ := formattest.Run(t, Format{}, fixture(t, "drift.json"), "-drift")
	if !strings.Contains(drift.Text, "drift:\n- aws_instance.bastion\n") {
		t.Errorf("-drift: %s", drift.Text)
	}
}

func TestConfigureRejectsBadCounts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		args []string
		msg  string
	}{
		{[]string{"-max-attrs", "x"}, "not an integer"},
		{[]string{"-max-attrs", "-1"}, "must be 0 or more"},
		{[]string{"-max-resources", "-5"}, "must be 0 or more"},
	}
	for _, tt := range tests {
		fs := flag.NewFlagSet("tfplan", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		(Format{}).Configure(fs)
		err := fs.Parse(tt.args)
		if err == nil || !strings.Contains(err.Error(), tt.msg) {
			t.Errorf("Parse(%v) error = %v, want %q", tt.args, err, tt.msg)
		}
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
	r, err := Squeeze(fixture(t, "noop-only.json"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(r.Data)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"changes":[]`, `"drift":[]`, `"outputs":[]`, `"tool":"terraform"`} {
		if !strings.Contains(string(out), key) {
			t.Errorf("data lacks %s: %s", key, out)
		}
	}
	created, err := Squeeze(fixture(t, "create-only.json"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	out, _ = json.Marshal(created.Data)
	for _, key := range []string{"previous_address", "deposed", "importing", "replaced_by", "attrs_omitted"} {
		if strings.Contains(string(out), `"`+key+`"`) {
			t.Errorf("data contains empty %s: %s", key, out)
		}
	}
}

func TestDataPayloadInvalidAction(t *testing.T) {
	t.Parallel()
	if _, err := json.Marshal(Report{Changes: []ResourceChange{{Address: "a", Action: Action(99)}}}); err == nil {
		t.Fatal("Marshal() with an invalid action returned no error")
	}
}

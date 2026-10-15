package ansible

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/formattest"
	"github.com/google/go-cmp/cmp"
)

const fixtureDir = "testdata"

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(fixtureDir, name, "input.*"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("fixture %s: %v %v", name, matches, err)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func squeeze(t *testing.T, input []byte, o Options) (core.Result, Report) {
	t.Helper()
	r, err := Squeeze(input, o)
	if err != nil {
		t.Fatal(err)
	}
	return r, r.Data.(Report)
}

func TestContract(t *testing.T) {
	t.Parallel()
	formattest.Contract(t, Format{}, fixtureDir)
}

func TestRealInputs(t *testing.T) {
	t.Parallel()
	formattest.Real(t, Format{}, "testdata/real")
}

const stars = " ****************************************"

func TestDetect(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input string
		want  bool
	}{
		"play and task":      {"\nPLAY [web]" + stars + "\n\nTASK [ping]" + stars + "\nok: [a]\n", true},
		"play and recap":     {"PLAY [web]" + stars + "\nskipping: no hosts matched\n", true},
		"unnamed play":       {"PLAY" + stars + "\nRUNNING HANDLER [x]" + stars + "\n", true},
		"color":              {"\x1b[0;32mPLAY [web]" + stars + "\x1b[0m\nTASK [ping]" + stars + "\n", true},
		"json callback":      {`{"custom_stats": {}, "global_custom_stats": {}, "plays": [], "stats": {}}`, true},
		"json with warning":  {"[WARNING]: x\n{\n    \"custom_stats\": {},\n    \"global_custom_stats\": {},\n    \"plays\": [\n", true},
		"play without task":  {"PLAY [web]" + stars + "\nhello\n", false},
		"banner no stars":    {"PLAY [web]\nTASK [ping]\n", false},
		"other json":         {`{"plays": [], "stats": {}}`, false},
		"eslint":             {`[{"filePath":"/a.js","messages":[],"errorCount":0}]`, false},
		"go test text":       {"ok  \texample.com/p\t0.1s\n", false},
		"pytest":             {"============================= test session starts ==============================\n", false},
		"github actions log": {"build\tRun\t2026-10-13T22:20:23.7522810Z PLAY [web]" + stars + "\n", false},
	}
	for name, tt := range tests {
		if got := (Format{}).Detect([]byte(tt.input)); got != tt.want {
			t.Errorf("%s: Detect() = %v, want %v", name, got, tt.want)
		}
	}
}

func TestParseRejectsOtherText(t *testing.T) {
	t.Parallel()
	if _, err := Squeeze([]byte("hello\nworld\n"), DefaultOptions()); !errors.Is(err, ErrNoRun) {
		t.Errorf("Squeeze() error = %v, want ErrNoRun", err)
	}
	if _, err := Squeeze([]byte(`{"custom_stats": {}, "global_custom_stats": {}, "plays": [`), DefaultOptions()); err == nil {
		t.Error("Squeeze() of a cut JSON document returned no error")
	}
}

func moduleArgs(v any, out map[string]bool, inside bool) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			moduleArgs(child, out, inside || k == "invocation")
		}
	case []any:
		for _, child := range x {
			moduleArgs(child, out, inside)
		}
	case string:
		if inside && len(x) >= 12 {
			out[x] = true
		}
	}
}

var blobStart = regexp.MustCompile(`(?m)=> \{$`)

func argValues(t *testing.T, input []byte) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	s := string(input)
	if strings.HasPrefix(s, "{") {
		var doc any
		if err := json.Unmarshal(input, &doc); err != nil {
			t.Fatal(err)
		}
		moduleArgs(doc, out, false)
		return out
	}
	for _, loc := range blobStart.FindAllStringIndex(s, -1) {
		start := loc[1] - 1
		end := strings.Index(s[start:], "\n}\n")
		var doc any
		if err := json.Unmarshal([]byte(s[start:start+end+2]), &doc); err != nil {
			t.Fatal(err)
		}
		moduleArgs(doc, out, false)
	}
	return out
}

func TestModuleArgsNeverPrinted(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"json", "verbose"} {
		input := fixture(t, name)
		values := argValues(t, input)
		if !values["echo migrate --password hunter2-module-arg-secret"] {
			t.Fatalf("%s: the fixture lost the module args of the migration task: %d values", name, len(values))
		}
		for _, o := range []Options{DefaultOptions(), {All: true}} {
			r, err := Squeeze(input, o)
			if err != nil {
				t.Fatal(err)
			}
			envelope, err := core.Envelope("ansible", len(input), r, nil)
			if err != nil {
				t.Fatal(err)
			}
			for v := range values {
				if strings.Contains(r.Text, v) || strings.Contains(string(envelope), v) {
					t.Errorf("%s: module arg %q reached the output", name, v)
				}
			}
			if strings.Contains(r.Text+string(envelope), "hunter2") {
				t.Errorf("%s: the password reached the output", name)
			}
		}
	}
}

func TestFailedModuleArgsNeverPrinted(t *testing.T) {
	t.Parallel()
	text := "PLAY [db]" + stars + "\n\nTASK [Create user]" + stars + "\n" +
		`fatal: [db1]: FAILED! => {"changed": false, "invocation": {"module_args": {"password": "s3cr3t-module-arg", "name": "app"}}, "msg": "role app does not exist"}` + "\n"
	doc := `{"custom_stats": {}, "global_custom_stats": {}, "plays": [{"play": {"name": "db"}, "tasks": [{"task": {"name": "Create user", "path": "/home/dev/shop/db.yml:4"}, "hosts": {"db1": {"failed": true, "invocation": {"module_args": {"password": "s3cr3t-module-arg"}}, "msg": "role app does not exist"}}}]}], "stats": {"db1": {"failures": 1}}}`
	for _, input := range []string{text, doc} {
		r, _ := squeeze(t, []byte(input), Options{All: true})
		if strings.Contains(r.Text, "s3cr3t") || !strings.Contains(r.Text, "msg: role app does not exist") {
			t.Errorf("text:\n%s", r.Text)
		}
		envelope, err := core.Envelope("ansible", len(input), r, nil)
		if err != nil || strings.Contains(string(envelope), "s3cr3t") {
			t.Errorf("envelope %v:\n%s", err, envelope)
		}
	}
}

func TestNoLogResultsStayCensored(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"failed", "verbose", "json"} {
		_, rep := squeeze(t, fixture(t, name), Options{All: true})
		found := false
		for _, task := range rep.Tasks {
			if task.Task != "Rotate the API token" {
				continue
			}
			found = true
			if !strings.Contains(task.Censored, "no_log: true") || task.Stdout != "" || task.Stderr != "" || task.RC != "" {
				t.Errorf("%s: censored task %+v", name, task)
			}
		}
		if !found {
			t.Errorf("%s: the no_log task is not listed with --all", name)
		}
	}
}

func statuses(rep Report) []string {
	var out []string
	for _, task := range rep.Tasks {
		label := string(task.Status) + " " + task.Host + ": " + task.Task
		if task.Item != "" {
			label += " " + task.Item
		}
		out = append(out, label)
	}
	return out
}

func TestToolSemanticsOfFailures(t *testing.T) {
	t.Parallel()
	text := []string{
		"unreachable web1: Check the app directory",
		"failed localhost: Assert the release is current",
		"changed localhost: Install the config",
		"rescued localhost: Fetch the cached build",
		"changed localhost: Reload shop",
		"changed localhost: Write the release file",
		"changed localhost: Run migrations",
		"changed localhost: Set the database password",
		"ignored localhost: Rotate the API token",
		"ignored localhost: Optional cleanup",
		"ignored localhost: Check each service port 9090",
	}
	for _, name := range []string{"failed", "verbose"} {
		_, rep := squeeze(t, fixture(t, name), Options{All: true})
		if diff := cmp.Diff(text, statuses(rep)); diff != "" {
			t.Errorf("%s (-want +got):\n%s", name, diff)
		}
		if rep.Changed != 5 || rep.Ignored != 3 || rep.Rescued != 1 || rep.Failed != 1 || rep.Unreachable != 1 {
			t.Errorf("%s: counts %+v", name, rep)
		}
	}
	_, rep := squeeze(t, fixture(t, "json"), Options{All: true})
	want := []string{
		"unreachable web1: Check the app directory",
		"failed localhost: Assert the release is current",
		"changed localhost: Install the config",
		"recovered localhost: Fetch the cached build",
		"changed localhost: Reload shop",
		"changed localhost: Write the release file",
		"changed localhost: Run migrations",
		"changed localhost: Set the database password",
		"recovered localhost: Rotate the API token",
		"recovered localhost: Optional cleanup",
		"recovered localhost: Check each service port 9090",
	}
	if diff := cmp.Diff(want, statuses(rep)); diff != "" {
		t.Errorf("json (-want +got):\n%s", diff)
	}
}

func jsonRunWith(stats string) string {
	return `{"custom_stats": {}, "global_custom_stats": {}, "plays": [{"play": {"name": "p"}, "tasks": [` +
		`{"task": {"name": "first"}, "hosts": {"h": {"failed": true, "msg": "one"}}},` +
		`{"task": {"name": "loop"}, "hosts": {"h": {"failed": true, "changed": true, "results": [{"item": "a", "failed": false}, {"_ansible_item_label": "b", "item": {"k": 1}, "failed": true, "msg": "two"}]}}},` +
		`{"task": {"name": "last"}, "hosts": {"h": {"failed": true, "msg": "three"}}}]}], "stats": {"h": ` + stats + `}}`
}

func TestJSONClassificationFollowsTheRecap(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		stats string
		want  []string
	}{
		"ignored":     {`{"failures": 1, "ignored": 2}`, []string{"failed h: last", "ignored h: first", "ignored h: loop b"}},
		"rescued":     {`{"failures": 1, "rescued": 2}`, []string{"failed h: last", "rescued h: first", "rescued h: loop b"}},
		"both":        {`{"failures": 1, "rescued": 1, "ignored": 1}`, []string{"failed h: last", "recovered h: first", "recovered h: loop b"}},
		"no recovery": {`{"failures": 3}`, []string{"failed h: first", "failed h: loop b", "failed h: last"}},
		"no failures": {`{"failures": 0}`, []string{"failed h: first", "failed h: loop b", "failed h: last"}},
	}
	for name, tt := range tests {
		_, rep := squeeze(t, []byte(jsonRunWith(tt.stats)), Options{All: true})
		if diff := cmp.Diff(tt.want, statuses(rep)); diff != "" {
			t.Errorf("%s (-want +got):\n%s", name, diff)
		}
	}
}

func TestRescueNeedsTheRecap(t *testing.T) {
	t.Parallel()
	input := "PLAY [p]" + stars + "\n\nTASK [first]" + stars + "\n" +
		`fatal: [h]: FAILED! => {"msg": "one"}` + "\n\nTASK [second]" + stars + "\n" +
		`fatal: [h]: FAILED! => {"msg": "two"}` + "\n"
	r, rep := squeeze(t, []byte(input), DefaultOptions())
	if diff := cmp.Diff([]string{"failed h: first", "failed h: second"}, statuses(rep)); diff != "" {
		t.Errorf("without a recap (-want +got):\n%s", diff)
	}
	if !strings.HasPrefix(r.Text, "ansible-playbook: 1 play, 1 failed, no PLAY RECAP\n") || !cmp.Equal(r.Conditions, []string{"error", "fail"}) {
		t.Errorf("conditions %v, text:\n%s", r.Conditions, r.Text)
	}
	recap := input + "\nPLAY RECAP" + stars + "\nh : ok=0 changed=0 unreachable=0 failed=1 skipped=0 rescued=0 ignored=0\n"
	_, rep = squeeze(t, []byte(recap), DefaultOptions())
	if diff := cmp.Diff([]string{"failed h: first", "failed h: second"}, statuses(rep)); diff != "" {
		t.Errorf("recap without rescued (-want +got):\n%s", diff)
	}
}

func TestErrorsAndWarningsAreNeverDropped(t *testing.T) {
	t.Parallel()
	input := " [WARNING]: old style warning\n[DEPRECATION WARNING]: x is deprecated\n" +
		"PLAY [p]" + stars + "\n\nTASK [t]" + stars + "\n[ERROR]: Could not load the callback\n\n" +
		"TASK [u]" + stars + "\nERROR! the role 'web' was not found\n"
	r, rep := squeeze(t, []byte(input), DefaultOptions())
	if diff := cmp.Diff([]string{"Could not load the callback", "the role 'web' was not found"}, rep.Errors); diff != "" {
		t.Errorf("errors (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"old style warning", "x is deprecated"}, rep.Warnings); diff != "" {
		t.Errorf("warnings (-want +got):\n%s", diff)
	}
	if !cmp.Equal(r.Conditions, []string{"error", "warning"}) || !strings.Contains(r.Text, "error: the role 'web' was not found\n") {
		t.Errorf("conditions %v, text:\n%s", r.Conditions, r.Text)
	}
}

func TestErrorMessageWhenTheResultHasNone(t *testing.T) {
	t.Parallel()
	input := "PLAY [p]" + stars + "\n\nTASK [t]" + stars + "\n[ERROR]: Task failed: boom\nOrigin: /home/dev/shop/x.yml:3:7\n\n" +
		"3     - name: t\n        ^ column 7\n\nfatal: [h]: FAILED! => {\"changed\": false}\n"
	_, rep := squeeze(t, []byte(input), DefaultOptions())
	want := []Task{{Status: StatusFailed, Host: "h", Play: "p", Task: "t", Path: "/home/dev/shop/x.yml:3:7", Detail: Detail{Msg: "Task failed: boom"}}}
	if diff := cmp.Diff(want, rep.Tasks); diff != "" || len(rep.Errors) != 0 {
		t.Errorf("errors %v, tasks (-want +got):\n%s", rep.Errors, diff)
	}
}

func TestLimits(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString("PLAY [p]" + stars + "\n\nTASK [t]" + stars + "\n")
	for _, h := range []string{"a", "b", "c", "d"} {
		b.WriteString("fatal: [" + h + `]: FAILED! => {"msg": "line 1\nline 2\nline 3"}` + "\n")
	}
	b.WriteString("\nPLAY RECAP" + stars + "\n")
	for _, h := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		failed := "0"
		if h < "e" {
			failed = "1"
		}
		b.WriteString(h + " : ok=1 changed=0 unreachable=0 failed=" + failed + " skipped=0 rescued=0 ignored=0\n")
	}
	r, rep := squeeze(t, []byte(b.String()), Options{MaxTasks: 2, MaxHosts: 1, MaxLines: 1})
	if len(rep.Hosts) != 5 || rep.HostsOmitted != 2 || len(rep.Tasks) != 2 || rep.TasksOmitted != 2 {
		t.Errorf("hosts %d omitted %d, tasks %d omitted %d", len(rep.Hosts), rep.HostsOmitted, len(rep.Tasks), rep.TasksOmitted)
	}
	for _, want := range []string{"  ... 2 hosts (--max-hosts 0)\n", "... 2 tasks (--max-tasks 0)\n", "    ... 2 lines cut (--max-lines 0)\n    line 3\n", "7 hosts, 4 failed\n"} {
		if !strings.Contains(r.Text, want) {
			t.Errorf("text lacks %q:\n%s", want, r.Text)
		}
	}
	if rep.Tasks[0].Msg != "line 1\nline 2\nline 3" {
		t.Errorf("data msg %q is not whole", rep.Tasks[0].Msg)
	}
	r, _ = squeeze(t, []byte(b.String()), Options{})
	if strings.Contains(r.Text, "... ") {
		t.Errorf("0 limits still cut:\n%s", r.Text)
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	text := "\x1b[0;32mPLAY [p\x1b[2J]" + stars + "\x1b[0m\n\nTASK [t\u0007]" + stars + "\n" +
		"fatal: [h\x1b]0;title\x07]: FAILED! => {\"msg\": \"bad\\u001b[31m\\nfatal: [x]: forged\\tline\", \"stderr\": \"a\\rb\"}\n" +
		"failed: [h] (item=\x1b[1mi) => {\"msg\": \"\\u0000x\"}\n"
	doc := `{"custom_stats": {}, "global_custom_stats": {}, "plays": [{"play": {"name": "p\n"}, "tasks": [{"task": {"name": "t\nfailed x: y", "path": "a\u001b[2Jb"}, "hosts": {"h\nq": {"failed": true, "msg": "m\u001b[31m", "results": [{"failed": true, "item": "i\nj"}]}}}]}], "stats": {}}`
	for _, input := range []string{text, doc} {
		for _, o := range []Options{DefaultOptions(), {All: true, MaxLines: 1}} {
			r, rep := squeeze(t, []byte(input), o)
			formattest.CheckText(t, r.Text)
			if len(rep.Tasks) == 0 {
				t.Errorf("no task parsed from %q", input)
			}
			for _, l := range strings.Split(r.Text, "\n") {
				if strings.HasPrefix(l, "fatal: ") || strings.HasPrefix(l, "failed x") {
					t.Errorf("forged line %q in:\n%s", l, r.Text)
				}
			}
		}
	}
}

func TestVersionAndPaths(t *testing.T) {
	t.Parallel()
	_, rep := squeeze(t, fixture(t, "verbose"), DefaultOptions())
	if rep.ToolVersion != "2.21.4" || rep.Tasks[1].Path != "/home/dev/shop/site.yml:74" {
		t.Errorf("version %q, path %q", rep.ToolVersion, rep.Tasks[1].Path)
	}
	_, rep = squeeze(t, []byte("ansible-playbook 2.9.27\n  config file = None\nPLAY [p]"+stars+"\nTASK [t]"+stars+"\n"), DefaultOptions())
	if rep.ToolVersion != "2.9.27" {
		t.Errorf("old version %q", rep.ToolVersion)
	}
}

func TestCheckModeMarker(t *testing.T) {
	t.Parallel()
	input := "PLAY [p] [CHECK MODE]" + stars + "\n\nTASK [t] [CHECK MODE]" + stars + "\nchanged: [h]\n"
	_, rep := squeeze(t, []byte(input), Options{All: true})
	if diff := cmp.Diff([]string{"changed h: t"}, statuses(rep)); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte("PLAY [p]" + stars + "\nTASK [t]" + stars + "\nfatal: [h]: UNREACHABLE! => {\n\"msg\": 1\n}\n...ignoring\n"))
	f.Add([]byte(jsonRunWith(`{"failures": 1, "rescued": 1, "ignored": 1}`)))
	f.Fuzz(func(t *testing.T, input []byte) {
		run, err := Parse(input)
		if err != nil {
			return
		}
		failures := 0
		for _, e := range run.Events {
			if e.Status == StatusFailed || e.Status == StatusUnreachable {
				failures++
			}
		}
		for _, o := range []Options{DefaultOptions(), {MaxLines: 1, MaxTasks: 1, MaxHosts: 1}, {All: true}} {
			rep := Build(run, o)
			res := core.Result{Text: Text(rep, o), Data: rep, Conditions: conditions(rep)}
			formattest.CheckText(t, res.Text)
			formattest.CheckNotices(t, res.Text)
			if again := Text(Build(run, o), o); again != res.Text {
				t.Fatalf("output differs between runs")
			}
			if o.All {
				listed := 0
				for _, task := range rep.Tasks {
					if task.Status != StatusChanged {
						listed++
					}
				}
				if listed != failures {
					t.Fatalf("%d failures in the input, %d listed with --all", failures, listed)
				}
			}
			if _, err := core.Envelope("ansible", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
	})
}

func TestJSONLoopItemFailures(t *testing.T) {
	t.Parallel()
	tests := map[string]bool{
		`{"failed": true, "rc": 0}`:                         true,
		`{"failed": false, "rc": 2}`:                        false,
		`{"failed_when_result": true, "rc": 0}`:             true,
		`{"failed_when_result": "x is defined"}`:            true,
		`{"failed_when_result": false, "rc": 1}`:            false,
		`{"rc": 2}`:                                         true,
		`{"rc": 0, "exception": "(traceback unavailable)"}`: true,
		`{"rc": 0}`: false,
		`{}`:        false,
	}
	for item, want := range tests {
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(item), &m); err != nil {
			t.Fatal(err)
		}
		if got := itemFailed(m); got != want {
			t.Errorf("itemFailed(%s) = %v, want %v", item, got, want)
		}
	}
}

func TestIgnoringMarksTheLastFailedHost(t *testing.T) {
	t.Parallel()
	input := "PLAY [p]" + stars + "\n\nTASK [t]" + stars + "\n" +
		"failed: [a] (item=1) => {\"msg\": \"a1\"}\n" +
		"failed: [b] (item=1) => {\"msg\": \"b1\"}\n" +
		"ok: [b] => (item=2)\n" +
		"failed: [a] (item=2) => {\"msg\": \"a2\"}\n" +
		"...ignoring\n" +
		"fatal: [c]: FAILED! => {\"msg\": \"c\"}\n" +
		"...ignoring\n"
	_, rep := squeeze(t, []byte(input), Options{All: true})
	want := []string{"failed b: t 1", "ignored a: t 1", "ignored a: t 2", "ignored c: t"}
	if diff := cmp.Diff(want, statuses(rep)); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

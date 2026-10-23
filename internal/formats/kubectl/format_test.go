package kubectl

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
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

func report(t *testing.T, input string) Report {
	t.Helper()
	r, err := Squeeze([]byte(input), Options{})
	if err != nil {
		t.Fatal(err)
	}
	return r.Data.(Report)
}

func only(t *testing.T, input string) Object {
	t.Helper()
	r := report(t, input)
	if len(r.Objects) != 1 {
		t.Fatalf("objects = %+v", r.Objects)
	}
	return r.Objects[0]
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
		"list":              {`{"apiVersion":"v1","items":[],"kind":"List","metadata":{}}`, true},
		"pretty object":     {"{\n    \"apiVersion\": \"v1\",\n    \"kind\": \"Pod\",\n    \"metadata\": {\"name\": \"a\"}\n}", true},
		"no metadata":       {`{"apiVersion":"v1","kind":"Status"}`, false},
		"kind first":        {`{"kind":"Pod","apiVersion":"v1","metadata":{}}`, false},
		"array":             {`[{"apiVersion":"v1","kind":"Pod","metadata":{}}]`, false},
		"sarif":             {`{"version":"2.1.0","runs":[]}`, false},
		"terraform outputs": {`{"bucket":{"sensitive":false,"type":"string","value":"x"}}`, false},
		"yaml":              {"apiVersion: v1\nkind: Pod\nmetadata:\n  name: a\n", false},
	}
	for name, tt := range tests {
		if got := (Format{}).Detect([]byte(tt.input)); got != tt.want {
			t.Errorf("%s: Detect() = %v, want %v", name, got, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"truncated":          `{"apiVersion":`,
		"array":              `[]`,
		"null":               `null`,
		"no kind":            `{"apiVersion":"v1","metadata":{}}`,
		"no api version":     `{"kind":"Pod","metadata":{}}`,
		"items not a list":   `{"apiVersion":"v1","kind":"List","items":{}}`,
		"item without kind":  `{"apiVersion":"v1","kind":"List","items":[{"metadata":{"name":"a"}}]}`,
		"null item":          `{"apiVersion":"v1","kind":"PodList","items":[null]}`,
		"metadata not a map": `{"apiVersion":"v1","kind":"Pod","metadata":[]}`,
	} {
		if _, err := Squeeze([]byte(input), DefaultOptions()); err == nil {
			t.Errorf("%s: Squeeze() returned no error", name)
		}
	}
}

func TestTypedListItemsTakeTheListKind(t *testing.T) {
	t.Parallel()
	r := report(t, `{"apiVersion":"v1","kind":"PodList","items":[{"metadata":{"name":"a","namespace":"n"},"status":{"phase":"Succeeded"}}]}`)
	if len(r.Objects) != 1 || r.Objects[0].Kind != "Pod" || r.Objects[0].Status != "Succeeded" {
		t.Errorf("objects = %+v", r.Objects)
	}
}

func TestConditions(t *testing.T) {
	t.Parallel()
	tests := map[string][]string{
		"mixed":         {"fail", "restarts"},
		"events":        {"warning"},
		"single-secret": {},
		"nodes":         {},
		"node-notready": {"fail"},
	}
	for name, want := range tests {
		r, err := Squeeze(fixture(t, name), DefaultOptions())
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(want, r.Conditions); diff != "" {
			t.Errorf("%s: conditions (-want +got):\n%s", name, diff)
		}
	}
}

func TestMixedFixture(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "mixed"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	data := r.Data.(Report)
	want := []string{
		"Pod shop/api-badimage ImagePullBackOff",
		"Pod shop/batch-pending Pending",
		"Pod shop/migrate-dvl69 Error",
		"Pod shop/migrate-gtc4v Error",
		"Pod shop/worker-56bff96fc9-c8xj6 CrashLoopBackOff",
		"Deployment shop/worker ",
		"Job shop/migrate Failed",
	}
	var got []string
	for _, o := range data.Objects {
		if o.Unhealthy {
			got = append(got, o.Kind+" "+o.Namespace+"/"+o.Name+" "+o.Status)
		}
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unhealthy objects (-want +got):\n%s", diff)
	}
	if data.ObjectsTotal != 13 || data.Unhealthy != 7 || data.Restarts != 3 {
		t.Errorf("counts = %d objects, %d unhealthy, %d restarts", data.ObjectsTotal, data.Unhealthy, data.Restarts)
	}
	if !strings.Contains(r.Text, "unhealthy Pod shop/worker-56bff96fc9-c8xj6: CrashLoopBackOff, 0/1 ready, 3 restarts; worker waiting CrashLoopBackOff, last Error exit 1\n") {
		t.Errorf("crash loop line missing:\n%s", r.Text)
	}
}

func TestPodStatusFollowsKubectl(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		pod        string
		status     string
		unhealthy  bool
		containers []string
	}{
		"init waiting": {
			`"spec":{"initContainers":[{"name":"i"}],"containers":[{"name":"c"}]},"status":{"phase":"Pending","initContainerStatuses":[{"name":"i","state":{"waiting":{"reason":"ImagePullBackOff"}}}]}`,
			"Init:ImagePullBackOff", true, []string{"init i waiting ImagePullBackOff"},
		},
		"init failed": {
			`"spec":{"initContainers":[{"name":"i"}],"containers":[{"name":"c"}]},"status":{"phase":"Pending","initContainerStatuses":[{"name":"i","restartCount":2,"state":{"terminated":{"exitCode":1}}}]}`,
			"Init:ExitCode:1", true, []string{"init i terminated ExitCode:1"},
		},
		"init failed with reason": {
			`"spec":{"initContainers":[{"name":"i"}],"containers":[{"name":"c"}]},"status":{"phase":"Pending","initContainerStatuses":[{"name":"i","state":{"terminated":{"reason":"Error","exitCode":2}}}]}`,
			"Init:Error", true, []string{"init i terminated Error exit 2"},
		},
		"init in progress": {
			`"spec":{"initContainers":[{"name":"i"},{"name":"j"}],"containers":[{"name":"c"}]},"status":{"phase":"Pending","initContainerStatuses":[{"name":"i","state":{"terminated":{"exitCode":0,"reason":"Completed"}}},{"name":"j","state":{"running":{}}}]}`,
			"Init:1/2", true, nil,
		},
		"sidecar started": {
			`"spec":{"initContainers":[{"name":"s","restartPolicy":"Always"}],"containers":[{"name":"c"}]},"status":{"phase":"Running","conditions":[{"type":"Initialized","status":"True"},{"type":"Ready","status":"True"}],"initContainerStatuses":[{"name":"s","ready":true,"started":true,"restartCount":1,"state":{"running":{}}}],"containerStatuses":[{"name":"c","ready":true,"state":{"running":{}}}]}`,
			"Running", false, []string{"s running"},
		},
		"signal": {
			`"spec":{"containers":[{"name":"c"}]},"status":{"phase":"Failed","containerStatuses":[{"name":"c","state":{"terminated":{"signal":9,"exitCode":137}}}]}`,
			"Signal:9", true, []string{"c terminated Signal:9"},
		},
		"oom killed": {
			`"spec":{"containers":[{"name":"c"}]},"status":{"phase":"Running","containerStatuses":[{"name":"c","restartCount":4,"lastState":{"terminated":{"reason":"OOMKilled","exitCode":137,"signal":9}},"state":{"running":{}}}]}`,
			"Running", true, []string{"c running, not ready, last OOMKilled signal 9"},
		},
		"completed beside running": {
			`"spec":{"containers":[{"name":"a"},{"name":"b"}]},"status":{"phase":"Running","conditions":[{"type":"Ready","status":"False"}],"containerStatuses":[{"name":"a","state":{"terminated":{"reason":"Completed","exitCode":0}}},{"name":"b","ready":true,"state":{"running":{}}}]}`,
			"Running", true, nil,
		},
		"completed beside running without ready condition": {
			`"spec":{"containers":[{"name":"a"},{"name":"b"}]},"status":{"phase":"Running","containerStatuses":[{"name":"a","state":{"terminated":{"reason":"Completed","exitCode":0}}},{"name":"b","ready":true,"state":{"running":{}}}]}`,
			"NotReady", true, nil,
		},
		"no state": {
			`"spec":{"containers":[{"name":"c"}]},"status":{"phase":"Pending","containerStatuses":[{"name":"c"}]}`,
			"Pending", true, []string{"c no state"},
		},
		"waiting without reason": {
			`"spec":{"containers":[{"name":"c"}]},"status":{"phase":"Pending","containerStatuses":[{"name":"c","state":{"waiting":{}}}]}`,
			"Pending", true, []string{"c waiting unknown"},
		},
		"evicted": {
			`"spec":{"containers":[{"name":"c"}]},"status":{"phase":"Failed","reason":"Evicted","message":"The node was low on resource: memory."}`,
			"Evicted", true, nil,
		},
		"scheduling gated": {
			`"spec":{"containers":[{"name":"c"}]},"status":{"phase":"Pending","conditions":[{"type":"PodScheduled","status":"False","reason":"SchedulingGated"}]}`,
			"SchedulingGated", true, nil,
		},
		"terminating": {
			`"metadata":{"name":"p","deletionTimestamp":"2026-01-01T00:00:00Z"},"spec":{"containers":[{"name":"c"}]},"status":{"phase":"Running","containerStatuses":[{"name":"c","ready":true,"state":{"running":{}}}]}`,
			"Terminating", false, nil,
		},
		"node lost": {
			`"metadata":{"name":"p","deletionTimestamp":"2026-01-01T00:00:00Z"},"spec":{"containers":[{"name":"c"}]},"status":{"phase":"Running","reason":"NodeLost"}`,
			"Unknown", true, nil,
		},
	}
	for name, tt := range tests {
		o := only(t, `{"apiVersion":"v1","kind":"Pod",`+tt.pod+`}`)
		if o.Status != tt.status || o.Unhealthy != tt.unhealthy || !cmp.Equal(o.Containers, tt.containers) {
			t.Errorf("%s: status %q unhealthy %v containers %q, want %q %v %q", name, o.Status, o.Unhealthy, o.Containers, tt.status, tt.unhealthy, tt.containers)
		}
	}
	if o := only(t, `{"apiVersion":"v1","kind":"Pod","spec":{"containers":[{"name":"c"}]},"status":{"phase":"Failed","reason":"Evicted","message":"low on memory"}}`); o.Message != "low on memory" {
		t.Errorf("evicted message = %q", o.Message)
	}
}

func TestWorkloads(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		object    string
		line      string
		unhealthy bool
	}{
		"deployment default replicas": {
			`"kind":"Deployment","metadata":{"name":"d","namespace":"n"},"status":{"readyReplicas":1,"updatedReplicas":1,"availableReplicas":1}`,
			"Deployment n/d: 1/1 ready, 1 updated, 1 available", false,
		},
		"deployment deadline": {
			`"kind":"Deployment","metadata":{"name":"d"},"spec":{"replicas":0},"status":{"conditions":[{"type":"Progressing","status":"False","reason":"ProgressDeadlineExceeded","message":"took too long"}]}`,
			"unhealthy Deployment d: 0/0 ready, 0 updated, 0 available, Progressing False ProgressDeadlineExceeded; took too long", true,
		},
		"replicaset failure": {
			`"kind":"ReplicaSet","metadata":{"name":"r"},"spec":{"replicas":0},"status":{"conditions":[{"type":"ReplicaFailure","status":"True","reason":"FailedCreate","message":"quota"}]}`,
			"unhealthy ReplicaSet r: 0/0 ready, 0 current, ReplicaFailure True FailedCreate; quota", true,
		},
		"statefulset": {
			`"kind":"StatefulSet","metadata":{"name":"s"},"spec":{"replicas":3},"status":{"readyReplicas":2,"updatedReplicas":3}`,
			"unhealthy StatefulSet s: 2/3 ready, 3 updated", true,
		},
		"daemonset misscheduled": {
			`"kind":"DaemonSet","metadata":{"name":"ds"},"status":{"desiredNumberScheduled":2,"numberReady":2,"numberAvailable":2,"updatedNumberScheduled":2,"numberMisscheduled":1}`,
			"unhealthy DaemonSet ds: 2/2 ready, 2 updated, 2 available, 1 misscheduled", true,
		},
		"job parallel": {
			`"kind":"Job","metadata":{"name":"j"},"spec":{"parallelism":3},"status":{"succeeded":1,"active":2}`,
			"Job j: Running, 1/1 of 3 succeeded, 2 active", false,
		},
		"job suspended": {
			`"kind":"Job","metadata":{"name":"j"},"status":{"conditions":[{"type":"Suspended","status":"True"}]}`,
			"Job j: Suspended, 0/1 succeeded", false,
		},
		"job failure target": {
			`"kind":"Job","metadata":{"name":"j"},"status":{"failed":1,"conditions":[{"type":"FailureTarget","status":"True","reason":"PodFailurePolicy"}]}`,
			"unhealthy Job j: FailureTarget, 0/1 succeeded, 1 failed, PodFailurePolicy", true,
		},
		"job terminating": {
			`"kind":"Job","metadata":{"name":"j","deletionTimestamp":"2026-01-01T00:00:00Z"},"spec":{"completions":2},"status":{}`,
			"Job j: Terminating, 0/2 succeeded", false,
		},
		"cronjob": {
			`"kind":"CronJob","metadata":{"name":"c"},"spec":{"suspend":true},"status":{"active":[{"name":"x"}],"lastScheduleTime":"2026-01-01T00:00:00Z"}`,
			"CronJob c: Suspended, 1 active, last schedule 2026-01-01T00:00:00Z", false,
		},
		"node without conditions": {
			`"kind":"Node","metadata":{"name":"n1"},"status":{}`,
			"unhealthy Node n1: Unknown", true,
		},
		"pvc pending": {
			`"kind":"PersistentVolumeClaim","metadata":{"name":"data","namespace":"n"},"status":{"phase":"Pending"}`,
			"unhealthy PersistentVolumeClaim n/data: Pending", true,
		},
		"pvc terminating": {
			`"kind":"PersistentVolumeClaim","metadata":{"name":"data","deletionTimestamp":"x"},"status":{"phase":"Bound"}`,
			"PersistentVolumeClaim data: Terminating", false,
		},
		"pv failed": {
			`"kind":"PersistentVolume","metadata":{"name":"pv"},"status":{"phase":"Failed","message":"recycle failed"}`,
			"unhealthy PersistentVolume pv: Failed; recycle failed", true,
		},
		"load balancer pending": {
			`"kind":"Service","metadata":{"name":"lb"},"spec":{"type":"LoadBalancer","clusterIP":"10.0.0.1","ports":[{"port":443,"targetPort":"https","nodePort":30443,"protocol":"TCP"}]},"status":{"loadBalancer":{}}`,
			"Service lb: LoadBalancer, 10.0.0.1, 443->https:30443/TCP, external IP pending", false,
		},
		"external name": {
			`"kind":"Service","metadata":{"name":"ext"},"spec":{"type":"ExternalName","externalName":"db.example.com"}`,
			"Service ext: ExternalName, db.example.com", false,
		},
		"custom resource not ready": {
			`"kind":"Certificate","metadata":{"name":"tls"},"status":{"conditions":[{"type":"Ready","status":"False","reason":"Pending","message":"issuing"}]}`,
			"unhealthy Certificate tls: Ready False Pending; issuing", true,
		},
		"custom resource unreadable status": {
			`"kind":"Widget","metadata":{"name":"w","deletionTimestamp":"x"},"status":"weird"`,
			"Widget w: terminating", false,
		},
		"namespace": {
			`"kind":"Namespace","metadata":{"name":"shop"},"status":{"phase":"Active"}`,
			"Namespace shop: Active", false,
		},
		"pod status of the wrong type": {
			`"kind":"Pod","metadata":{"name":"p"},"status":{"phase":7}`,
			"unhealthy Pod p: unreadable status", true,
		},
		"service account": {
			`"kind":"ServiceAccount","metadata":{"name":"default"}`,
			"ServiceAccount default", false,
		},
	}
	for name, tt := range tests {
		o := only(t, `{"apiVersion":"v1",`+tt.object+`}`)
		if got := line(o); got != tt.line || o.Unhealthy != tt.unhealthy {
			t.Errorf("%s: line %q unhealthy %v, want %q %v", name, got, o.Unhealthy, tt.line, tt.unhealthy)
		}
	}
}

func TestStatusDecodeErrors(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"Deployment", "DaemonSet", "Job", "CronJob", "Node", "PersistentVolumeClaim", "Service"} {
		for _, field := range []string{"spec", "status"} {
			if kind == "DaemonSet" && field == "spec" || kind == "PersistentVolumeClaim" && field == "spec" {
				continue
			}
			o := only(t, `{"apiVersion":"v1","kind":"`+kind+`","metadata":{"name":"x"},"`+field+`":[]}`)
			if !o.Unhealthy || o.Status != "unreadable status" {
				t.Errorf("%s with a bad %s: %+v", kind, field, o)
			}
		}
	}
}

func TestEvents(t *testing.T) {
	t.Parallel()
	input := `{"apiVersion":"v1","kind":"List","items":[
		{"apiVersion":"v1","kind":"Event","type":"Warning","reason":"BackOff","message":"m","count":2,"lastTimestamp":"2026-01-01T00:00:01Z","involvedObject":{"kind":"Pod","namespace":"n","name":"p"}},
		{"apiVersion":"v1","kind":"Event","type":"Warning","reason":"BackOff","message":"m","count":3,"lastTimestamp":"2026-01-01T00:00:05Z","involvedObject":{"kind":"Pod","namespace":"n","name":"p"}},
		{"apiVersion":"events.k8s.io/v1","kind":"Event","type":"Warning","reason":"FailedMount","note":"volume x","deprecatedCount":4,"series":{"count":6,"lastObservedTime":"2026-01-01T00:00:09Z"},"regarding":{"kind":"Pod","namespace":"n","name":"q"}},
		{"apiVersion":"v1","kind":"Event","type":"Warning","eventTime":"2026-01-01T00:00:00Z"},
		{"apiVersion":"v1","kind":"Event","type":"Normal","reason":"Pulled","message":"ok"}]}`
	r, err := Squeeze([]byte(input), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	want := "kubectl: 5 events, 4 warnings\n" +
		"warning Pod n/q FailedMount: volume x (x6)\n" +
		"warning Pod n/p BackOff: m (x5)\n" +
		"warning no reason\n"
	if r.Text != want {
		t.Errorf("text:\n%s\nwant:\n%s", r.Text, want)
	}
	cut, err := Squeeze([]byte(input), Options{MaxEvents: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(cut.Text, "... 2 more warning events: BackOff 1, no reason 1 (--max-events 0)\n") {
		t.Errorf("cut text:\n%s", cut.Text)
	}
}

func TestLimitsCountEveryObject(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "all-namespaces"), Options{MaxObjects: 5})
	if err != nil {
		t.Fatal(err)
	}
	data := r.Data.(Report)
	sum := 0
	for _, c := range data.OmittedByKind {
		sum += c.Count
	}
	if len(data.Objects) != 5 || data.ObjectsOmitted != data.ObjectsTotal-5 || sum != data.ObjectsOmitted || data.UnhealthyCut != data.Unhealthy-5 {
		t.Errorf("listed %d, omitted %d (by kind %d, unhealthy %d) of %d", len(data.Objects), data.ObjectsOmitted, sum, data.UnhealthyCut, data.ObjectsTotal)
	}
	for _, o := range data.Objects {
		if !o.Unhealthy {
			t.Errorf("a healthy object is listed before a cut unhealthy one: %+v", o)
		}
	}
}

func TestLongMessagesAreCutInTextOnly(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("x", 500)
	r, err := Squeeze([]byte(`{"apiVersion":"v1","kind":"PersistentVolume","metadata":{"name":"pv"},"status":{"phase":"Failed","message":"`+long+`"}}`), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.Text, long) || !strings.Contains(r.Text, "...") {
		t.Errorf("text keeps the whole message:\n%s", r.Text)
	}
	if r.Data.(Report).Objects[0].Message != long {
		t.Errorf("data lost the message")
	}
}

type visit func(s string) string

func walk(v any, kind string, f visit) any {
	switch t := v.(type) {
	case map[string]any:
		if k, ok := t["kind"].(string); ok {
			kind = k
		}
		for key, child := range t {
			switch {
			case key == "items" && strings.HasSuffix(kind, "List"):
				t[key] = walk(child, strings.TrimSuffix(kind, "List"), f)
			case key == "metadata":
				if m, ok := child.(map[string]any); ok {
					if a, ok := m["annotations"]; ok {
						m["annotations"] = values(a, f)
					}
				}
				t[key] = walk(child, kind, f)
			case (kind == "Secret" && (key == "data" || key == "stringData")) || (kind == "ConfigMap" && (key == "data" || key == "binaryData")):
				t[key] = values(child, f)
			case key == "env":
				if list, ok := child.([]any); ok {
					for _, e := range list {
						if m, ok := e.(map[string]any); ok {
							if s, ok := m["value"].(string); ok {
								m["value"] = f(s)
							}
						}
					}
				}
			default:
				t[key] = walk(child, kind, f)
			}
		}
	case []any:
		for i := range t {
			t[i] = walk(t[i], kind, f)
		}
	}
	return v
}

func values(v any, f visit) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	for k, child := range m {
		if s, ok := child.(string); ok {
			m[k] = f(s)
		}
	}
	return m
}

func rewrite(input []byte, f visit) ([]byte, bool) {
	dec := json.NewDecoder(bytes.NewReader(input))
	dec.UseNumber()
	var doc any
	if dec.Decode(&doc) != nil {
		return nil, false
	}
	out, err := json.Marshal(walk(doc, "", f))
	return out, err == nil
}

func sensitive(t *testing.T, input []byte) []string {
	t.Helper()
	var found []string
	add := func(s string) {
		if len(s) >= 6 {
			found = append(found, s)
		}
	}
	if _, ok := rewrite(input, func(s string) string {
		add(s)
		if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
			add(string(decoded))
		}
		var embedded any
		if json.Unmarshal([]byte(s), &embedded) == nil {
			raw, _ := json.Marshal(embedded)
			found = append(found, sensitive(t, raw)...)
		}
		return s
	}); !ok {
		t.Fatal("input is not JSON")
	}
	return found
}

func TestSensitiveValuesNeverReachOutput(t *testing.T) {
	t.Parallel()
	checked := map[string]bool{}
	for _, c := range formattest.Cases(t, fixtureDir) {
		values := sensitive(t, c.Input)
		r, err := Squeeze(c.Input, Options{})
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := core.Envelope("kubectl", len(c.Input), r, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range values {
			checked[v] = true
			if strings.Contains(r.Text, v) || strings.Contains(string(envelope), v) || strings.Contains(string(envelope), strings.TrimSuffix(strings.TrimPrefix(string(mustJSON(t, v)), `"`), `"`)) {
				t.Errorf("%s: a sensitive value reached the output: %q", c.Name, v)
			}
		}
	}
	for _, v := range []string{"hunter2-sentinel-pw", "aHVudGVyMi1zZW50aW5lbC1wdw==", "shop-admin-sentinel", "env-value-sentinel-token", "debug-configmap-sentinel", "configmap-annotation-sentinel"} {
		if !checked[v] {
			t.Errorf("the fixtures hold no sensitive value %q, so the test checks nothing for it", v)
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestOutputDoesNotDependOnSensitiveValues(t *testing.T) {
	t.Parallel()
	for _, c := range formattest.Cases(t, fixtureDir) {
		changed, ok := rewrite(c.Input, func(s string) string { return "changed-" + s })
		if !ok {
			t.Fatal("input is not JSON")
		}
		before, err := Squeeze(c.Input, Options{})
		if err != nil {
			t.Fatal(err)
		}
		after, err := Squeeze(changed, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(before.Data, after.Data); diff != "" {
			t.Errorf("%s: data depends on a sensitive value (-before +after):\n%s", c.Name, diff)
		}
	}
}

func TestInputStringsCannotForgeLines(t *testing.T) {
	t.Parallel()
	input := []byte(`{"apiVersion":"v1","kind":"List","items":[
		{"apiVersion":"v1","kind":"Pod\nunhealthy forged","metadata":{"name":"p\nunhealthy forged","namespace":"n\u001b[31m"},"status":{"phase":"Failed\nunhealthy forged"}},
		{"apiVersion":"v1","kind":"Pod","metadata":{"name":"q"},"spec":{"containers":[{"name":"c"}]},"status":{"phase":"Pending","message":"m\nunhealthy forged","containerStatuses":[{"name":"c\nunhealthy forged","state":{"waiting":{"reason":"r\nunhealthy forged"}}}]}},
		{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"c"},"data":{"k\nunhealthy forged":"v"}},
		{"apiVersion":"v1","kind":"Event","type":"Warning","reason":"r\nwarning forged","message":"m\nwarning forged","involvedObject":{"kind":"K\nwarning forged","name":"x"}}]}`)
	r, err := Squeeze(input, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	formattest.CheckText(t, r.Text)
	for _, line := range strings.Split(r.Text, "\n") {
		if strings.HasPrefix(line, "unhealthy forged") || strings.HasPrefix(line, "warning forged") {
			t.Errorf("input forged a line: %q\n%s", line, r.Text)
		}
	}
}

func TestDataPayload(t *testing.T) {
	t.Parallel()
	r, err := Squeeze(fixture(t, "empty"), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	out, err := core.Envelope("kubectl", 10, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "null") {
		t.Errorf("envelope has a null collection: %s", out)
	}
}

func FuzzSqueeze(f *testing.F) {
	for _, data := range formattest.Inputs(f, fixtureDir) {
		f.Add(data)
	}
	f.Add([]byte(`{"apiVersion":"v1","kind":"List","items":[{"apiVersion":"v1","kind":"Secret","metadata":{"name":"s","annotations":{"a":"s"}},"data":{"k":"s"}},{"apiVersion":"v1","kind":"Event","type":"Warning","count":-3}]}`))
	f.Fuzz(func(t *testing.T, input []byte) {
		formattest.CheckStable(t, func() (core.Result, error) { return Squeeze(input, DefaultOptions()) })
		raws, err := Parse(input)
		if err != nil {
			return
		}
		objects, warnings := 0, 0
		for _, r := range raws {
			if r.Kind == "Event" {
				if f, _ := event(r); f.Type == "Warning" {
					warnings++
				}
				continue
			}
			objects++
		}
		for _, o := range []Options{DefaultOptions(), {MaxObjects: 1, MaxEvents: 1}} {
			r := Build(raws, o)
			res := core.Result{Text: Text(r), Data: r, Conditions: conditions(r)}
			formattest.CheckText(t, res.Text)
			formattest.CheckNotices(t, res.Text)
			if len(r.Objects)+r.ObjectsOmitted != objects || r.ObjectsTotal != objects {
				t.Fatalf("%d objects listed and %d omitted, %d in the input", len(r.Objects), r.ObjectsOmitted, objects)
			}
			if r.Warnings != warnings || len(r.Events)+r.EventsOmitted > warnings {
				t.Fatalf("%d warning events listed and %d omitted, %d in the input", len(r.Events), r.EventsOmitted, warnings)
			}
			if r.ObjectsOmitted > 0 && !strings.Contains(res.Text, "(--max-objects 0)") || r.EventsOmitted > 0 && !strings.Contains(res.Text, "(--max-events 0)") {
				t.Fatalf("items cut without a notice:\n%s", res.Text)
			}
			if _, err := core.Envelope("kubectl", len(input), res, nil); err != nil {
				t.Fatalf("Envelope() error: %v", err)
			}
		}
		changed, ok := rewrite(input, func(s string) string { return s + "-changed" })
		if !ok {
			return
		}
		before, errBefore := Squeeze(input, Options{})
		after, errAfter := Squeeze(changed, Options{})
		if errBefore != nil || errAfter != nil {
			return
		}
		if before.Text != after.Text {
			t.Fatalf("text depends on a sensitive value:\n%s\n---\n%s", before.Text, after.Text)
		}
	})
}

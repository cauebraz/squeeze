package kubectl

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type terminated struct {
	Reason   string `json:"reason"`
	ExitCode int    `json:"exitCode"`
	Signal   int    `json:"signal"`
}

type containerState struct {
	Waiting *struct {
		Reason string `json:"reason"`
	} `json:"waiting"`
	Running    *struct{}   `json:"running"`
	Terminated *terminated `json:"terminated"`
}

type containerStatus struct {
	Name         string         `json:"name"`
	Ready        bool           `json:"ready"`
	RestartCount int            `json:"restartCount"`
	Started      *bool          `json:"started"`
	State        containerState `json:"state"`
	LastState    containerState `json:"lastState"`
}

type container struct {
	Name          string `json:"name"`
	RestartPolicy string `json:"restartPolicy"`
}

type podSpec struct {
	Containers     []container `json:"containers"`
	InitContainers []container `json:"initContainers"`
}

type podStatus struct {
	Phase                 string            `json:"phase"`
	Reason                string            `json:"reason"`
	Message               string            `json:"message"`
	Conditions            []condition       `json:"conditions"`
	ContainerStatuses     []containerStatus `json:"containerStatuses"`
	InitContainerStatuses []containerStatus `json:"initContainerStatuses"`
}

func find(conds []condition, typ string) *condition {
	for i := range conds {
		if conds[i].Type == typ {
			return &conds[i]
		}
	}
	return nil
}

func isTrue(conds []condition, typ string) bool {
	c := find(conds, typ)
	return c != nil && c.Status == "True"
}

func exit(t *terminated) string {
	reason := t.Reason
	switch {
	case reason == "" && t.Signal != 0:
		return fmt.Sprintf("Signal:%d", t.Signal)
	case reason == "":
		return fmt.Sprintf("ExitCode:%d", t.ExitCode)
	case t.Signal != 0:
		return fmt.Sprintf("%s signal %d", reason, t.Signal)
	}
	return fmt.Sprintf("%s exit %d", reason, t.ExitCode)
}

func containerProblem(c containerStatus, init bool, podRestarts int) string {
	var parts []string
	switch {
	case c.State.Waiting != nil:
		parts = append(parts, "waiting "+orUnknown(c.State.Waiting.Reason))
	case c.State.Terminated != nil:
		t := c.State.Terminated
		if t.ExitCode == 0 && t.Signal == 0 && (init || t.Reason == "Completed") && c.RestartCount == 0 {
			return ""
		}
		parts = append(parts, "terminated "+exit(t))
	case c.State.Running != nil && !c.Ready && !init:
		parts = append(parts, "running, not ready")
	case c.State.Running != nil:
		if c.RestartCount == 0 {
			return ""
		}
		parts = append(parts, "running")
	default:
		parts = append(parts, "no state")
	}
	if c.LastState.Terminated != nil {
		parts = append(parts, "last "+exit(c.LastState.Terminated))
	}
	if c.RestartCount > 0 && c.RestartCount != podRestarts {
		parts = append(parts, fmt.Sprintf("%d %s", c.RestartCount, plural(c.RestartCount, "restart", "restarts")))
	}
	prefix := c.Name
	if init {
		prefix = "init " + prefix
	}
	return prefix + " " + strings.Join(parts, ", ")
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func pod(r Raw, o *Object) error {
	var spec podSpec
	var st podStatus
	if err := r.decode("spec", &spec); err != nil {
		return err
	}
	if err := r.decode("status", &st); err != nil {
		return err
	}
	restartable := map[string]bool{}
	total := len(spec.Containers)
	for _, c := range spec.InitContainers {
		if c.RestartPolicy == "Always" {
			restartable[c.Name] = true
			total++
		}
	}
	reason := st.Phase
	if st.Reason != "" {
		reason = st.Reason
	}
	if c := find(st.Conditions, "PodScheduled"); c != nil && c.Reason == "SchedulingGated" {
		reason = "SchedulingGated"
	}
	ready, restarts, sidecarRestarts := 0, 0, 0
	initializing := false
	for i, c := range st.InitContainerStatuses {
		restarts += c.RestartCount
		if restartable[c.Name] {
			sidecarRestarts += c.RestartCount
		}
		t := c.State.Terminated
		switch {
		case t != nil && t.ExitCode == 0:
			continue
		case restartable[c.Name] && c.Started != nil && *c.Started:
			if c.Ready {
				ready++
			}
			continue
		case t != nil:
			reason = "Init:" + exit(t)
			if t.Reason != "" {
				reason = "Init:" + t.Reason
			}
		case c.State.Waiting != nil && c.State.Waiting.Reason != "" && c.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + c.State.Waiting.Reason
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(spec.InitContainers))
		}
		initializing = true
		break
	}
	if !initializing || isTrue(st.Conditions, "Initialized") {
		restarts = sidecarRestarts
		running := false
		for i := len(st.ContainerStatuses) - 1; i >= 0; i-- {
			c := st.ContainerStatuses[i]
			restarts += c.RestartCount
			switch {
			case c.State.Waiting != nil && c.State.Waiting.Reason != "":
				reason = c.State.Waiting.Reason
			case c.State.Terminated != nil && c.State.Terminated.Reason != "":
				reason = c.State.Terminated.Reason
			case c.State.Terminated != nil:
				reason = exit(c.State.Terminated)
			case c.Ready && c.State.Running != nil:
				running = true
				ready++
			}
		}
		if reason == "Completed" && running {
			reason = "NotReady"
			if find(st.Conditions, "Ready") != nil {
				reason = "Running"
			}
		}
	}
	if r.Meta.DeletionTimestamp != "" && st.Reason == "NodeLost" {
		reason = "Unknown"
	} else if r.Meta.DeletionTimestamp != "" && st.Phase != "Succeeded" && st.Phase != "Failed" {
		reason = "Terminating"
	}
	o.Status = reason
	o.Restarts = restarts
	o.Details = append(o.Details, fmt.Sprintf("%d/%d ready", ready, total))
	if restarts > 0 {
		o.Details = append(o.Details, fmt.Sprintf("%d %s", restarts, plural(restarts, "restart", "restarts")))
	}
	for _, c := range st.InitContainerStatuses {
		if p := containerProblem(c, !restartable[c.Name], restarts); p != "" {
			o.Containers = append(o.Containers, p)
		}
	}
	for _, c := range st.ContainerStatuses {
		if p := containerProblem(c, false, restarts); p != "" {
			o.Containers = append(o.Containers, p)
		}
	}
	switch st.Phase {
	case "Succeeded":
	case "Running":
		o.Unhealthy = ready < total
	default:
		o.Unhealthy = true
	}
	o.Message = st.Message
	if c := find(st.Conditions, "PodScheduled"); c != nil && c.Status == "False" {
		o.Details = append(o.Details, orUnknown(c.Reason))
		if o.Message == "" {
			o.Message = c.Message
		}
	}
	return nil
}

type replicas struct {
	Replicas *int `json:"replicas"`
}

func desired(r Raw) (int, error) {
	var spec replicas
	if err := r.decode("spec", &spec); err != nil {
		return 0, err
	}
	if spec.Replicas == nil {
		return 1, nil
	}
	return *spec.Replicas, nil
}

type workloadStatus struct {
	Replicas               int         `json:"replicas"`
	ReadyReplicas          int         `json:"readyReplicas"`
	UpdatedReplicas        int         `json:"updatedReplicas"`
	AvailableReplicas      int         `json:"availableReplicas"`
	CurrentReplicas        int         `json:"currentReplicas"`
	DesiredNumberScheduled int         `json:"desiredNumberScheduled"`
	CurrentNumberScheduled int         `json:"currentNumberScheduled"`
	NumberReady            int         `json:"numberReady"`
	UpdatedNumberScheduled int         `json:"updatedNumberScheduled"`
	NumberAvailable        int         `json:"numberAvailable"`
	NumberMisscheduled     int         `json:"numberMisscheduled"`
	Conditions             []condition `json:"conditions"`
}

func failing(o *Object, conds []condition, typ, bad string) {
	if c := find(conds, typ); c != nil && c.Status == bad {
		o.Unhealthy = true
		o.Details = append(o.Details, typ+" "+bad+" "+orUnknown(c.Reason))
		if o.Message == "" {
			o.Message = c.Message
		}
	}
}

func workload(r Raw, o *Object) error {
	var st workloadStatus
	if err := r.decode("status", &st); err != nil {
		return err
	}
	want := st.DesiredNumberScheduled
	if r.Kind != "DaemonSet" {
		n, err := desired(r)
		if err != nil {
			return err
		}
		want = n
	}
	switch r.Kind {
	case "Deployment":
		o.Details = append(o.Details, fmt.Sprintf("%d/%d ready", st.ReadyReplicas, want), fmt.Sprintf("%d updated", st.UpdatedReplicas), fmt.Sprintf("%d available", st.AvailableReplicas))
		o.Unhealthy = st.ReadyReplicas < want || st.UpdatedReplicas < want || st.AvailableReplicas < want
		failing(o, st.Conditions, "Progressing", "False")
		failing(o, st.Conditions, "ReplicaFailure", "True")
	case "StatefulSet":
		o.Details = append(o.Details, fmt.Sprintf("%d/%d ready", st.ReadyReplicas, want), fmt.Sprintf("%d updated", st.UpdatedReplicas))
		o.Unhealthy = st.ReadyReplicas < want
	case "ReplicaSet", "ReplicationController":
		o.Details = append(o.Details, fmt.Sprintf("%d/%d ready", st.ReadyReplicas, want), fmt.Sprintf("%d current", st.Replicas))
		o.Unhealthy = st.ReadyReplicas < want
		failing(o, st.Conditions, "ReplicaFailure", "True")
	default:
		o.Details = append(o.Details, fmt.Sprintf("%d/%d ready", st.NumberReady, want), fmt.Sprintf("%d updated", st.UpdatedNumberScheduled), fmt.Sprintf("%d available", st.NumberAvailable))
		o.Unhealthy = st.NumberReady < want || st.NumberAvailable < want
		if st.NumberMisscheduled > 0 {
			o.Unhealthy = true
			o.Details = append(o.Details, fmt.Sprintf("%d misscheduled", st.NumberMisscheduled))
		}
	}
	return nil
}

type jobSpec struct {
	Completions *int `json:"completions"`
	Parallelism *int `json:"parallelism"`
}

type jobStatus struct {
	Succeeded  int         `json:"succeeded"`
	Failed     int         `json:"failed"`
	Active     int         `json:"active"`
	Conditions []condition `json:"conditions"`
}

func job(r Raw, o *Object) error {
	var spec jobSpec
	var st jobStatus
	if err := r.decode("spec", &spec); err != nil {
		return err
	}
	if err := r.decode("status", &st); err != nil {
		return err
	}
	switch {
	case isTrue(st.Conditions, "Complete"):
		o.Status = "Complete"
	case isTrue(st.Conditions, "Failed"):
		o.Status = "Failed"
	case r.Meta.DeletionTimestamp != "":
		o.Status = "Terminating"
	case isTrue(st.Conditions, "Suspended"):
		o.Status = "Suspended"
	case isTrue(st.Conditions, "FailureTarget"):
		o.Status = "FailureTarget"
	default:
		o.Status = "Running"
	}
	switch {
	case spec.Completions != nil:
		o.Details = append(o.Details, fmt.Sprintf("%d/%d succeeded", st.Succeeded, *spec.Completions))
	case spec.Parallelism != nil && *spec.Parallelism > 1:
		o.Details = append(o.Details, fmt.Sprintf("%d/1 of %d succeeded", st.Succeeded, *spec.Parallelism))
	default:
		o.Details = append(o.Details, fmt.Sprintf("%d/1 succeeded", st.Succeeded))
	}
	if st.Failed > 0 {
		o.Details = append(o.Details, fmt.Sprintf("%d failed", st.Failed))
	}
	if st.Active > 0 {
		o.Details = append(o.Details, fmt.Sprintf("%d active", st.Active))
	}
	if o.Status == "Failed" || o.Status == "FailureTarget" {
		o.Unhealthy = true
		c := find(st.Conditions, o.Status)
		o.Details = append(o.Details, orUnknown(c.Reason))
		o.Message = c.Message
	}
	return nil
}

func cronJob(r Raw, o *Object) error {
	var spec struct {
		Suspend bool `json:"suspend"`
	}
	var st struct {
		Active           []json.RawMessage `json:"active"`
		LastScheduleTime string            `json:"lastScheduleTime"`
	}
	if err := r.decode("spec", &spec); err != nil {
		return err
	}
	if err := r.decode("status", &st); err != nil {
		return err
	}
	if spec.Suspend {
		o.Status = "Suspended"
	}
	o.Details = append(o.Details, fmt.Sprintf("%d active", len(st.Active)))
	if st.LastScheduleTime != "" {
		o.Details = append(o.Details, "last schedule "+st.LastScheduleTime)
	}
	return nil
}

func node(r Raw, o *Object) error {
	var spec struct {
		Unschedulable bool `json:"unschedulable"`
	}
	var st struct {
		Conditions []condition `json:"conditions"`
		NodeInfo   struct {
			KubeletVersion string `json:"kubeletVersion"`
		} `json:"nodeInfo"`
	}
	if err := r.decode("spec", &spec); err != nil {
		return err
	}
	if err := r.decode("status", &st); err != nil {
		return err
	}
	status := []string{"Unknown"}
	if c := find(st.Conditions, "Ready"); c != nil {
		status[0] = "NotReady"
		if c.Status == "True" {
			status[0] = "Ready"
		}
		if c.Status != "True" {
			o.Message = c.Message
		}
	}
	o.Unhealthy = status[0] != "Ready"
	if spec.Unschedulable {
		status = append(status, "SchedulingDisabled")
	}
	o.Status = strings.Join(status, ",")
	for _, c := range st.Conditions {
		if c.Type != "Ready" && c.Status == "True" {
			o.Unhealthy = true
			o.Details = append(o.Details, c.Type+" "+orUnknown(c.Reason))
			if o.Message == "" {
				o.Message = c.Message
			}
		}
	}
	if st.NodeInfo.KubeletVersion != "" {
		o.Details = append(o.Details, "kubelet "+st.NodeInfo.KubeletVersion)
	}
	return nil
}

func volume(r Raw, o *Object) error {
	var st struct {
		Phase   string `json:"phase"`
		Message string `json:"message"`
	}
	if err := r.decode("status", &st); err != nil {
		return err
	}
	o.Status = st.Phase
	if r.Meta.DeletionTimestamp != "" {
		o.Status = "Terminating"
	}
	o.Message = st.Message
	if r.Kind == "PersistentVolumeClaim" {
		o.Unhealthy = st.Phase != "Bound"
	} else {
		o.Unhealthy = st.Phase == "Failed"
	}
	return nil
}

type servicePort struct {
	Port       int             `json:"port"`
	TargetPort json.RawMessage `json:"targetPort"`
	NodePort   int             `json:"nodePort"`
	Protocol   string          `json:"protocol"`
}

func service(r Raw, o *Object) error {
	var spec struct {
		Type         string        `json:"type"`
		ClusterIP    string        `json:"clusterIP"`
		ExternalName string        `json:"externalName"`
		Ports        []servicePort `json:"ports"`
	}
	var st struct {
		LoadBalancer struct {
			Ingress []json.RawMessage `json:"ingress"`
		} `json:"loadBalancer"`
	}
	if err := r.decode("spec", &spec); err != nil {
		return err
	}
	if err := r.decode("status", &st); err != nil {
		return err
	}
	o.Status = spec.Type
	if spec.ClusterIP != "" {
		o.Details = append(o.Details, spec.ClusterIP)
	}
	if spec.ExternalName != "" {
		o.Details = append(o.Details, spec.ExternalName)
	}
	var ports []string
	for _, p := range spec.Ports {
		s := strconv.Itoa(p.Port)
		if t := targetPort(p.TargetPort); t != "" && t != s {
			s += "->" + t
		}
		if p.NodePort != 0 {
			s += ":" + strconv.Itoa(p.NodePort)
		}
		if p.Protocol != "" {
			s += "/" + p.Protocol
		}
		ports = append(ports, s)
	}
	if len(ports) > 0 {
		o.Details = append(o.Details, strings.Join(ports, " "))
	}
	if spec.Type == "LoadBalancer" && len(st.LoadBalancer.Ingress) == 0 {
		o.Details = append(o.Details, "external IP pending")
	}
	return nil
}

func targetPort(raw json.RawMessage) string {
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return strconv.Itoa(n)
	}
	return text(raw)
}

func generic(r Raw, o *Object) {
	var st struct {
		Phase      string      `json:"phase"`
		Conditions []condition `json:"conditions"`
	}
	if json.Unmarshal(r.Fields["status"], &st) != nil {
		return
	}
	o.Status = st.Phase
	o.Unhealthy = st.Phase == "Failed"
	for _, typ := range []string{"Ready", "Available"} {
		failing(o, st.Conditions, typ, "False")
	}
}

func object(r Raw) (Object, error) {
	o := Object{Kind: r.Kind, Namespace: r.Meta.Namespace, Name: r.Meta.Name, Details: []string{}}
	var err error
	switch r.Kind {
	case "Pod":
		err = pod(r, &o)
	case "Deployment", "StatefulSet", "ReplicaSet", "ReplicationController", "DaemonSet":
		err = workload(r, &o)
	case "Job":
		err = job(r, &o)
	case "CronJob":
		err = cronJob(r, &o)
	case "Node":
		err = node(r, &o)
	case "PersistentVolumeClaim", "PersistentVolume":
		err = volume(r, &o)
	case "Service":
		err = service(r, &o)
	case "Secret":
		o.Status = text(r.Fields["type"])
		o.Keys = r.keys("data", "stringData")
	case "ConfigMap":
		o.Keys = r.keys("data", "binaryData")
	default:
		generic(r, &o)
	}
	if err != nil {
		return Object{}, err
	}
	if r.Meta.DeletionTimestamp != "" && o.Status != "Terminating" {
		o.Details = append(o.Details, "terminating")
	}
	sort.Strings(o.Keys)
	return o, nil
}

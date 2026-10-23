package kubectl

import (
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

type rawTerminated struct {
	Reason   string `json:"reason"`
	ExitCode int    `json:"exitCode"`
	Signal   int    `json:"signal"`
}

type rawContainerState struct {
	Waiting *struct {
		Reason string `json:"reason"`
	} `json:"waiting"`
	Running    *struct{}      `json:"running"`
	Terminated *rawTerminated `json:"terminated"`
}

type rawContainerStatus struct {
	Name         string            `json:"name"`
	Ready        bool              `json:"ready"`
	RestartCount int               `json:"restartCount"`
	Started      *bool             `json:"started"`
	State        rawContainerState `json:"state"`
	LastState    rawContainerState `json:"lastState"`
}

type podPhase string

const (
	podPending   podPhase = "Pending"
	podRunning   podPhase = "Running"
	podSucceeded podPhase = "Succeeded"
	podFailed    podPhase = "Failed"
	podUnknown   podPhase = "Unknown"
)

type restartPolicy string

const (
	restartAlways    restartPolicy = "Always"
	restartOnFailure restartPolicy = "OnFailure"
	restartNever     restartPolicy = "Never"
)

type rawContainer struct {
	Name          string        `json:"name"`
	RestartPolicy restartPolicy `json:"restartPolicy"`
}

type rawPodSpec struct {
	Containers     []rawContainer `json:"containers"`
	InitContainers []rawContainer `json:"initContainers"`
}

type rawPodStatus struct {
	Phase                 podPhase             `json:"phase"`
	Reason                string               `json:"reason"`
	Message               string               `json:"message"`
	Conditions            []rawCondition       `json:"conditions"`
	ContainerStatuses     []rawContainerStatus `json:"containerStatuses"`
	InitContainerStatuses []rawContainerStatus `json:"initContainerStatuses"`
}

func exit(t *rawTerminated) string {
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

func containerProblem(c rawContainerStatus, init bool, podRestarts int) string {
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
		parts = append(parts, fmt.Sprintf("%d %s", c.RestartCount, core.Plural(c.RestartCount, "restart", "restarts")))
	}
	prefix := c.Name
	if init {
		prefix = "init " + prefix
	}
	return prefix + " " + strings.Join(parts, ", ")
}

func pod(r rawObject, o *object) error {
	var spec rawPodSpec
	var st rawPodStatus
	if err := r.decode("spec", &spec); err != nil {
		return err
	}
	if err := r.decode("status", &st); err != nil {
		return err
	}
	restartable := map[string]bool{}
	total := len(spec.Containers)
	for _, c := range spec.InitContainers {
		if c.RestartPolicy == restartAlways {
			restartable[c.Name] = true
			total++
		}
	}
	reason := string(st.Phase)
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
	} else if r.Meta.DeletionTimestamp != "" && st.Phase != podSucceeded && st.Phase != podFailed {
		reason = "Terminating"
	}
	o.Status = reason
	o.Restarts = restarts
	o.Details = append(o.Details, fmt.Sprintf("%d/%d ready", ready, total))
	if restarts > 0 {
		o.Details = append(o.Details, fmt.Sprintf("%d %s", restarts, core.Plural(restarts, "restart", "restarts")))
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
	case podSucceeded:
	case podRunning:
		o.Unhealthy = ready < total
	case podPending, podFailed, podUnknown:
		o.Unhealthy = true
	default:
		o.Unhealthy = true
	}
	o.Message = st.Message
	if c := find(st.Conditions, "PodScheduled"); c != nil && c.Status == conditionFalse {
		o.Details = append(o.Details, orUnknown(c.Reason))
		if o.Message == "" {
			o.Message = c.Message
		}
	}
	return nil
}

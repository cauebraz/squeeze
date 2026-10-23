package kubectl

import (
	"encoding/json"
	"fmt"
)

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

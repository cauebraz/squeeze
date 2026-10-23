package kubectl

import "encoding/json"

type conditionStatus string

const (
	conditionTrue    conditionStatus = "True"
	conditionFalse   conditionStatus = "False"
	conditionUnknown conditionStatus = "Unknown"
)

type volumePhase string

const (
	volumePending   volumePhase = "Pending"
	volumeAvailable volumePhase = "Available"
	volumeBound     volumePhase = "Bound"
	volumeReleased  volumePhase = "Released"
	volumeFailed    volumePhase = "Failed"
	volumeLost      volumePhase = "Lost"
)

type rawCondition struct {
	Type    string          `json:"type"`
	Status  conditionStatus `json:"status"`
	Reason  string          `json:"reason"`
	Message string          `json:"message"`
}

func find(conds []rawCondition, typ string) *rawCondition {
	for i := range conds {
		if conds[i].Type == typ {
			return &conds[i]
		}
	}
	return nil
}

func trueCondition(conds []rawCondition, typ string) *rawCondition {
	if c := find(conds, typ); c != nil && c.Status == conditionTrue {
		return c
	}
	return nil
}

func isTrue(conds []rawCondition, typ string) bool {
	return trueCondition(conds, typ) != nil
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func failing(o *object, conds []rawCondition, typ string, bad conditionStatus) {
	if c := find(conds, typ); c != nil && c.Status == bad {
		o.Unhealthy = true
		o.Details = append(o.Details, typ+" "+string(bad)+" "+orUnknown(c.Reason))
		if o.Message == "" {
			o.Message = c.Message
		}
	}
}

func volume(r rawObject, o *object) error {
	var st struct {
		Phase   volumePhase `json:"phase"`
		Message string      `json:"message"`
	}
	if err := r.decode("status", &st); err != nil {
		return err
	}
	o.Status = string(st.Phase)
	if r.Meta.DeletionTimestamp != "" {
		o.Status = "Terminating"
	}
	o.Message = st.Message
	if r.Kind == "PersistentVolumeClaim" {
		o.Unhealthy = st.Phase != volumeBound
	} else {
		o.Unhealthy = st.Phase == volumeFailed
	}
	return nil
}

func generic(r rawObject, o *object) {
	var st struct {
		Phase      string         `json:"phase"`
		Conditions []rawCondition `json:"conditions"`
	}
	if json.Unmarshal(r.Fields["status"], &st) != nil {
		return
	}
	o.Status = st.Phase
	o.Unhealthy = st.Phase == "Failed"
	for _, typ := range []string{"Ready", "Available"} {
		failing(o, st.Conditions, typ, conditionFalse)
	}
}

func toObject(r rawObject) (object, error) {
	o := object{Kind: r.Kind, Namespace: r.Meta.Namespace, Name: r.Meta.Name, Details: []string{}}
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
		o.Status = jsonString(r.Fields["type"])
		o.Keys = r.keys("data", "stringData")
	case "ConfigMap":
		o.Keys = r.keys("data", "binaryData")
	default:
		generic(r, &o)
	}
	if err != nil {
		return object{}, err
	}
	if r.Meta.DeletionTimestamp != "" && o.Status != "Terminating" {
		o.Details = append(o.Details, "terminating")
	}
	return o, nil
}

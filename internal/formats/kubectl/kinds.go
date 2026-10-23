package kubectl

import (
	"encoding/json"
	"sort"
)

type condition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
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

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
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

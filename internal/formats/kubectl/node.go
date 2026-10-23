package kubectl

import "strings"

func node(r rawObject, o *object) error {
	var spec struct {
		Unschedulable bool `json:"unschedulable"`
	}
	var st struct {
		Conditions []rawCondition `json:"conditions"`
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
		if c.Status == conditionTrue {
			status[0] = "Ready"
		}
		if c.Status != conditionTrue {
			o.Message = c.Message
		}
	}
	o.Unhealthy = status[0] != "Ready"
	if spec.Unschedulable {
		status = append(status, "SchedulingDisabled")
	}
	o.Status = strings.Join(status, ",")
	for _, c := range st.Conditions {
		if c.Type != "Ready" && c.Status == conditionTrue {
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

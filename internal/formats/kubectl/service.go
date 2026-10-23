package kubectl

import (
	"encoding/json"
	"strconv"
	"strings"
)

type serviceType string

const (
	serviceClusterIP    serviceType = "ClusterIP"
	serviceNodePort     serviceType = "NodePort"
	serviceLoadBalancer serviceType = "LoadBalancer"
	serviceExternalName serviceType = "ExternalName"
)

type rawServicePort struct {
	Port       int             `json:"port"`
	TargetPort json.RawMessage `json:"targetPort"`
	NodePort   int             `json:"nodePort"`
	Protocol   string          `json:"protocol"`
}

func service(r rawObject, o *object) error {
	var spec struct {
		Type         serviceType      `json:"type"`
		ClusterIP    string           `json:"clusterIP"`
		ExternalName string           `json:"externalName"`
		Ports        []rawServicePort `json:"ports"`
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
	o.Status = string(spec.Type)
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
	if spec.Type == serviceLoadBalancer && len(st.LoadBalancer.Ingress) == 0 {
		o.Details = append(o.Details, "external IP pending")
	}
	return nil
}

func targetPort(raw json.RawMessage) string {
	var n int
	if json.Unmarshal(raw, &n) == nil {
		return strconv.Itoa(n)
	}
	return jsonString(raw)
}

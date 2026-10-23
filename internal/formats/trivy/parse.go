package trivy

import (
	"errors"
	"fmt"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/logscan"
)

var errNotTrivy = errors.New("input has no trivy SchemaVersion or ClusterName")

type rawVulnerability struct {
	VulnerabilityID  string `json:"VulnerabilityID"`
	PkgName          string `json:"PkgName"`
	InstalledVersion string `json:"InstalledVersion"`
	FixedVersion     string `json:"FixedVersion"`
	Status           string `json:"Status"`
	Severity         string `json:"Severity"`
	Title            string `json:"Title"`
	PrimaryURL       string `json:"PrimaryURL"`
}

type rawCause struct {
	Resource  string `json:"Resource"`
	StartLine int    `json:"StartLine"`
	EndLine   int    `json:"EndLine"`
}

type rawMisconfiguration struct {
	ID            string   `json:"ID"`
	AVDID         string   `json:"AVDID"`
	Title         string   `json:"Title"`
	Message       string   `json:"Message"`
	Resolution    string   `json:"Resolution"`
	Severity      string   `json:"Severity"`
	Status        string   `json:"Status"`
	PrimaryURL    string   `json:"PrimaryURL"`
	CauseMetadata rawCause `json:"CauseMetadata"`
}

type rawSecret struct {
	RuleID    string `json:"RuleID"`
	Category  string `json:"Category"`
	Severity  string `json:"Severity"`
	Title     string `json:"Title"`
	StartLine int    `json:"StartLine"`
	EndLine   int    `json:"EndLine"`
}

type rawLicense struct {
	Severity string `json:"Severity"`
	Category string `json:"Category"`
	PkgName  string `json:"PkgName"`
	FilePath string `json:"FilePath"`
	Name     string `json:"Name"`
}

type rawSummary struct {
	Successes int `json:"Successes"`
	Failures  int `json:"Failures"`
}

type rawResult struct {
	Target            string                `json:"Target"`
	Class             string                `json:"Class"`
	Type              string                `json:"Type"`
	Vulnerabilities   []rawVulnerability    `json:"Vulnerabilities"`
	MisconfSummary    *rawSummary           `json:"MisconfSummary"`
	Misconfigurations []rawMisconfiguration `json:"Misconfigurations"`
	Secrets           []rawSecret           `json:"Secrets"`
	Licenses          []rawLicense          `json:"Licenses"`
}

type rawK8sResource struct {
	Namespace string      `json:"Namespace"`
	Kind      string      `json:"Kind"`
	Name      string      `json:"Name"`
	Results   []rawResult `json:"Results"`
	Error     string      `json:"Error"`
}

type rawReport struct {
	SchemaVersion int `json:"SchemaVersion"`
	Trivy         struct {
		Version string `json:"Version"`
	} `json:"Trivy"`
	ArtifactName string `json:"ArtifactName"`
	ArtifactType string `json:"ArtifactType"`
	Metadata     struct {
		OS *struct {
			Family string `json:"Family"`
			Name   string `json:"Name"`
		} `json:"OS"`
	} `json:"Metadata"`
	Results     []rawResult           `json:"Results"`
	ClusterName *string               `json:"ClusterName"`
	Resources   []rawK8sResource      `json:"Resources"`
	Findings    []rawK8sResource      `json:"Findings"`
	Trailing    []logscan.FlaggedLine `json:"-"`
}

func parse(input []byte) (*rawReport, error) {
	var r rawReport
	rest, err := core.DecodeJSON(input, &r)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	r.Trailing = logscan.FlaggedLines(rest)
	if r.SchemaVersion == 0 {
		if r.ClusterName == nil {
			return nil, errNotTrivy
		}
		return &r, nil
	}
	if r.SchemaVersion != 2 {
		return nil, fmt.Errorf("unsupported SchemaVersion %d", r.SchemaVersion)
	}
	return &r, nil
}

package trivy

import (
	"errors"
	"fmt"

	"github.com/cauebraz/squeeze/internal/core"
)

var ErrNotTrivy = errors.New("input has no trivy SchemaVersion or ClusterName")

type Vulnerability struct {
	VulnerabilityID  string `json:"VulnerabilityID"`
	PkgName          string `json:"PkgName"`
	InstalledVersion string `json:"InstalledVersion"`
	FixedVersion     string `json:"FixedVersion"`
	Status           string `json:"Status"`
	Severity         string `json:"Severity"`
	Title            string `json:"Title"`
	PrimaryURL       string `json:"PrimaryURL"`
}

type Cause struct {
	Resource  string `json:"Resource"`
	StartLine int    `json:"StartLine"`
	EndLine   int    `json:"EndLine"`
}

type Misconfiguration struct {
	ID            string `json:"ID"`
	AVDID         string `json:"AVDID"`
	Title         string `json:"Title"`
	Message       string `json:"Message"`
	Resolution    string `json:"Resolution"`
	Severity      string `json:"Severity"`
	Status        string `json:"Status"`
	PrimaryURL    string `json:"PrimaryURL"`
	CauseMetadata Cause  `json:"CauseMetadata"`
}

type Secret struct {
	RuleID    string `json:"RuleID"`
	Category  string `json:"Category"`
	Severity  string `json:"Severity"`
	Title     string `json:"Title"`
	StartLine int    `json:"StartLine"`
	EndLine   int    `json:"EndLine"`
}

type License struct {
	Severity string `json:"Severity"`
	Category string `json:"Category"`
	PkgName  string `json:"PkgName"`
	FilePath string `json:"FilePath"`
	Name     string `json:"Name"`
}

type Summary struct {
	Successes int `json:"Successes"`
	Failures  int `json:"Failures"`
}

type Result struct {
	Target            string             `json:"Target"`
	Class             string             `json:"Class"`
	Type              string             `json:"Type"`
	Vulnerabilities   []Vulnerability    `json:"Vulnerabilities"`
	MisconfSummary    *Summary           `json:"MisconfSummary"`
	Misconfigurations []Misconfiguration `json:"Misconfigurations"`
	Secrets           []Secret           `json:"Secrets"`
	Licenses          []License          `json:"Licenses"`
}

type K8sResource struct {
	Namespace string   `json:"Namespace"`
	Kind      string   `json:"Kind"`
	Name      string   `json:"Name"`
	Results   []Result `json:"Results"`
	Error     string   `json:"Error"`
}

type RawReport struct {
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
	Results     []Result           `json:"Results"`
	ClusterName *string            `json:"ClusterName"`
	Resources   []K8sResource      `json:"Resources"`
	Findings    []K8sResource      `json:"Findings"`
	Trailing    []core.FlaggedLine `json:"-"`
}

func Parse(input []byte) (*RawReport, error) {
	var r RawReport
	rest, err := core.DecodeJSON(input, &r)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	r.Trailing = core.FlaggedLines(rest)
	if r.SchemaVersion == 0 {
		if r.ClusterName == nil {
			return nil, ErrNotTrivy
		}
		return &r, nil
	}
	if r.SchemaVersion != 2 {
		return nil, fmt.Errorf("unsupported SchemaVersion %d", r.SchemaVersion)
	}
	return &r, nil
}

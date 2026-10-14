package trivy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrNotTrivy = errors.New("input has no trivy SchemaVersion")

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
	Results []Result `json:"Results"`
}

func Parse(input []byte) (*RawReport, error) {
	var r RawReport
	if err := json.NewDecoder(bytes.NewReader(input)).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if r.SchemaVersion == 0 {
		return nil, ErrNotTrivy
	}
	if r.SchemaVersion != 2 {
		return nil, fmt.Errorf("unsupported SchemaVersion %d", r.SchemaVersion)
	}
	return &r, nil
}

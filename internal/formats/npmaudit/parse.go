package npmaudit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
)

var ErrNotAudit = errors.New("input has no npm audit auditReportVersion and vulnerabilities")

type Advisory struct {
	Source   int      `json:"source"`
	Name     string   `json:"name"`
	Title    string   `json:"title"`
	URL      string   `json:"url"`
	Severity string   `json:"severity"`
	CWE      []string `json:"cwe"`
	CVSS     struct {
		Score float64 `json:"score"`
	} `json:"cvss"`
	Range string `json:"range"`
}

type Via struct {
	Package  string
	Advisory *Advisory
}

func (v *Via) UnmarshalJSON(b []byte) error {
	switch {
	case string(b) == "null":
		return nil
	case b[0] == '"':
		return json.Unmarshal(b, &v.Package)
	}
	v.Advisory = &Advisory{}
	return json.Unmarshal(b, v.Advisory)
}

type Fix struct {
	Available   bool   `json:"available"`
	Force       bool   `json:"force,omitempty"`
	Name        string `json:"name,omitempty"`
	Version     string `json:"version,omitempty"`
	SemVerMajor bool   `json:"semver_major,omitempty"`
}

func (f *Fix) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case "true":
		*f = Fix{Available: true}
		return nil
	case "false", "null":
		*f = Fix{}
		return nil
	}
	var o struct {
		Name          string `json:"name"`
		Version       string `json:"version"`
		IsSemVerMajor bool   `json:"isSemVerMajor"`
	}
	if err := json.Unmarshal(b, &o); err != nil {
		return err
	}
	*f = Fix{Available: true, Force: true, Name: o.Name, Version: o.Version, SemVerMajor: o.IsSemVerMajor}
	return nil
}

type Vulnerability struct {
	Name         string   `json:"name"`
	Severity     string   `json:"severity"`
	IsDirect     bool     `json:"isDirect"`
	Via          []Via    `json:"via"`
	Effects      []string `json:"effects"`
	Range        string   `json:"range"`
	Nodes        []string `json:"nodes"`
	FixAvailable Fix      `json:"fixAvailable"`
}

type Dependencies struct {
	Prod         int `json:"prod"`
	Dev          int `json:"dev"`
	Optional     int `json:"optional"`
	Peer         int `json:"peer"`
	PeerOptional int `json:"peer_optional"`
	Total        int `json:"total"`
}

type Audit struct {
	Version         int
	Vulnerabilities map[string]Vulnerability
	Dependencies    Dependencies
}

type npmError struct {
	Code    string `json:"code"`
	Summary string `json:"summary"`
	Detail  string `json:"detail"`
}

func Parse(input []byte) (*Audit, error) {
	var doc struct {
		AuditReportVersion *int                     `json:"auditReportVersion"`
		Vulnerabilities    map[string]Vulnerability `json:"vulnerabilities"`
		Metadata           struct {
			Dependencies struct {
				Prod, Dev, Optional, Peer, Total int
				PeerOptional                     int `json:"peerOptional"`
			} `json:"dependencies"`
		} `json:"metadata"`
		Message string    `json:"message"`
		Error   *npmError `json:"error"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(input), &doc); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if doc.Error != nil {
		var parts []string
		for _, s := range []string{doc.Error.Code, doc.Error.Summary, doc.Error.Detail, doc.Message} {
			if s = core.Clean(s); s != "" {
				parts = append(parts, s)
			}
		}
		return nil, fmt.Errorf("npm reported an error: %s", strings.Join(parts, "; "))
	}
	if doc.AuditReportVersion == nil || doc.Vulnerabilities == nil {
		return nil, ErrNotAudit
	}
	if *doc.AuditReportVersion != 2 {
		return nil, fmt.Errorf("unsupported auditReportVersion %d", *doc.AuditReportVersion)
	}
	d := doc.Metadata.Dependencies
	deps := Dependencies{Prod: d.Prod, Dev: d.Dev, Optional: d.Optional, Peer: d.Peer, PeerOptional: d.PeerOptional, Total: d.Total}
	return &Audit{Version: 2, Vulnerabilities: doc.Vulnerabilities, Dependencies: deps}, nil
}

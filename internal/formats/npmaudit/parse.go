package npmaudit

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cauebraz/squeeze/internal/core"
	"github.com/cauebraz/squeeze/internal/logscan"
	"github.com/cauebraz/squeeze/internal/sanitize"
)

var errNotAudit = errors.New("input has no npm audit auditReportVersion and vulnerabilities")

type rawAdvisory struct {
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

type rawVia struct {
	Package  string
	Advisory *rawAdvisory
}

func (v *rawVia) UnmarshalJSON(b []byte) error {
	switch {
	case string(b) == "null":
		return nil
	case b[0] == '"':
		return json.Unmarshal(b, &v.Package)
	}
	v.Advisory = &rawAdvisory{}
	return json.Unmarshal(b, v.Advisory)
}

type rawFix struct {
	Available     bool   `json:"-"`
	Force         bool   `json:"-"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	IsSemVerMajor bool   `json:"isSemVerMajor"`
}

func (f *rawFix) UnmarshalJSON(b []byte) error {
	switch string(b) {
	case "true":
		*f = rawFix{Available: true}
		return nil
	case "false", "null":
		*f = rawFix{}
		return nil
	}
	type object rawFix
	var o object
	if err := json.Unmarshal(b, &o); err != nil {
		return err
	}
	*f = rawFix(o)
	f.Available, f.Force = true, true
	return nil
}

type rawVulnerability struct {
	Name         string   `json:"name"`
	Severity     string   `json:"severity"`
	IsDirect     bool     `json:"isDirect"`
	Via          []rawVia `json:"via"`
	Effects      []string `json:"effects"`
	Range        string   `json:"range"`
	Nodes        []string `json:"nodes"`
	FixAvailable rawFix   `json:"fixAvailable"`
}

type dependencies struct {
	Prod         int `json:"prod"`
	Dev          int `json:"dev"`
	Optional     int `json:"optional"`
	Peer         int `json:"peer"`
	PeerOptional int `json:"peer_optional"`
	Total        int `json:"total"`
}

type audit struct {
	Version         int
	Vulnerabilities map[string]rawVulnerability
	Dependencies    dependencies
	Trailing        []logscan.FlaggedLine
}

type rawError struct {
	Code    string `json:"code"`
	Summary string `json:"summary"`
	Detail  string `json:"detail"`
}

func parse(input []byte) (*audit, error) {
	var doc struct {
		AuditReportVersion *int                        `json:"auditReportVersion"`
		Vulnerabilities    map[string]rawVulnerability `json:"vulnerabilities"`
		Metadata           struct {
			Dependencies struct {
				Prod, Dev, Optional, Peer, Total int
				PeerOptional                     int `json:"peerOptional"`
			} `json:"dependencies"`
		} `json:"metadata"`
		Message string    `json:"message"`
		Error   *rawError `json:"error"`
	}
	lead, body := logscan.LeadingLines(input)
	rest, err := core.DecodeJSON(body, &doc)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if doc.Error != nil {
		var parts []string
		for _, s := range []string{doc.Error.Code, doc.Error.Summary, doc.Error.Detail, doc.Message} {
			if s = sanitize.Clean(s); s != "" {
				parts = append(parts, s)
			}
		}
		return nil, fmt.Errorf("npm reported an error: %s", strings.Join(parts, "; "))
	}
	if doc.AuditReportVersion == nil || doc.Vulnerabilities == nil {
		return nil, errNotAudit
	}
	if *doc.AuditReportVersion != 2 {
		return nil, fmt.Errorf("unsupported auditReportVersion %d", *doc.AuditReportVersion)
	}
	d := doc.Metadata.Dependencies
	deps := dependencies{Prod: d.Prod, Dev: d.Dev, Optional: d.Optional, Peer: d.Peer, PeerOptional: d.PeerOptional, Total: d.Total}
	return &audit{Version: 2, Vulnerabilities: doc.Vulnerabilities, Dependencies: deps, Trailing: append(logscan.FlaggedLines(lead), logscan.FlaggedLines(rest)...)}, nil
}

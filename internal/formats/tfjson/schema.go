package tfjson

import (
	"encoding/json"

	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type lineHeader struct {
	Level      string `json:"@level"`
	Message    string `json:"@message"`
	Type       string `json:"type"`
	AtTestFile string `json:"@testfile"`
	AtTestRun  string `json:"@testrun"`
	Terraform  string `json:"terraform"`
}

type linePayload struct {
	Diagnostic  *tfdiag.Raw             `json:"diagnostic"`
	Change      *streamChange           `json:"change"`
	Changes     *streamSummary          `json:"changes"`
	Hook        *streamHook             `json:"hook"`
	Outputs     map[string]streamOutput `json:"outputs"`
	Deferred    *streamDeferred         `json:"deferred_change"`
	TestRun     *streamTestRun          `json:"test_run"`
	TestFile    *streamTestFile         `json:"test_file"`
	TestSummary *streamTestSummary      `json:"test_summary"`
	Result      string                  `json:"result"`
	Policy      string                  `json:"policy_address"`
	Target      string                  `json:"target_address"`
	PolicyMeta  *struct {
		EnforcementLevel string `json:"enforcement_level"`
	} `json:"policy_metadata"`
}

type streamResource struct {
	Addr string `json:"addr"`
}

type streamChange struct {
	Resource         streamResource  `json:"resource"`
	PreviousResource *streamResource `json:"previous_resource"`
	Action           string          `json:"action"`
	Reason           string          `json:"reason"`
	Importing        *struct {
		ID string `json:"id"`
	} `json:"importing"`
}

type streamSummary struct {
	Add              int    `json:"add"`
	Change           int    `json:"change"`
	Import           int    `json:"import"`
	Remove           int    `json:"remove"`
	ActionInvocation int    `json:"action_invocation"`
	ActionFail       int    `json:"action_fail"`
	Operation        string `json:"operation"`
}

type streamHook struct {
	Resource streamResource  `json:"resource"`
	Action   json.RawMessage `json:"action"`
	Elapsed  float64         `json:"elapsed_seconds"`
}

type streamOutput struct {
	Sensitive bool            `json:"sensitive"`
	Value     json.RawMessage `json:"value"`
	Action    string          `json:"action"`
}

type streamDeferred struct {
	Reason string        `json:"reason"`
	Change *streamChange `json:"change"`
}

type streamTestRun struct {
	Path     string `json:"path"`
	Run      string `json:"run"`
	Progress string `json:"progress"`
	Status   string `json:"status"`
	Elapsed  *int64 `json:"elapsed"`
}

type streamTestFile struct {
	Path     string `json:"path"`
	Progress string `json:"progress"`
	Status   string `json:"status"`
}

type streamTestSummary struct {
	Status  string `json:"status"`
	Passed  int    `json:"passed"`
	Failed  int    `json:"failed"`
	Errored int    `json:"errored"`
	Skipped int    `json:"skipped"`
}

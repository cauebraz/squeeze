package tfjson

import (
	"encoding/json"

	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type Change struct {
	Address         string `json:"address"`
	PreviousAddress string `json:"previous_address,omitempty"`
	Action          string `json:"action"`
	Reason          string `json:"reason,omitempty"`
	ImportID        string `json:"import_id,omitempty"`
}

type Deferred struct {
	Address string `json:"address"`
	Action  string `json:"action"`
	Reason  string `json:"reason"`
}

type ActionInvocation struct {
	Address  string `json:"address"`
	Event    string `json:"event,omitempty"`
	Resource string `json:"resource,omitempty"`
	Invoked  bool   `json:"invoked,omitempty"`
}

type ListBlock struct {
	Address      string `json:"address"`
	ResourceType string `json:"resource_type,omitempty"`
	Total        int    `json:"total"`
	Complete     bool   `json:"complete"`
}

type FoundResource struct {
	Address     string                     `json:"address"`
	DisplayName string                     `json:"display_name"`
	Identity    map[string]json.RawMessage `json:"identity,omitempty"`
}

type Failure struct {
	Type    string `json:"type"`
	Address string `json:"address,omitempty"`
	Message string `json:"message"`
}

type PolicyResult struct {
	Policy      string `json:"policy"`
	Target      string `json:"target,omitempty"`
	Result      string `json:"result"`
	Outcome     string `json:"outcome"`
	Enforcement string `json:"enforcement,omitempty"`
}

type Timing struct {
	Address string  `json:"address"`
	Seconds float64 `json:"seconds"`
}

type Applied struct {
	Count   int      `json:"count"`
	Slowest []Timing `json:"slowest"`
}

type Output struct {
	Name      string          `json:"name"`
	Action    string          `json:"action,omitempty"`
	Sensitive bool            `json:"sensitive,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
}

type Summary struct {
	Operation        string `json:"operation"`
	Add              int    `json:"add"`
	Change           int    `json:"change"`
	Remove           int    `json:"remove"`
	Import           int    `json:"import"`
	ActionInvocation int    `json:"action_invocation"`
	ActionFail       int    `json:"action_fail"`
}

type TestRun struct {
	File        string              `json:"file"`
	Run         string              `json:"run"`
	Status      string              `json:"status"`
	ElapsedMs   *int64              `json:"elapsed_ms,omitempty"`
	Diagnostics []tfdiag.Diagnostic `json:"diagnostics"`
}

type TestFile struct {
	File        string              `json:"file"`
	Status      string              `json:"status"`
	Diagnostics []tfdiag.Diagnostic `json:"diagnostics"`
}

type LeftResource struct {
	Address    string `json:"address"`
	DeposedKey string `json:"deposed_key,omitempty"`
}

type LeftInState struct {
	File      string         `json:"file"`
	Run       string         `json:"run,omitempty"`
	Reason    string         `json:"reason"`
	State     string         `json:"state,omitempty"`
	Resources []LeftResource `json:"resources"`
}

type Tests struct {
	Status  string        `json:"status"`
	Passed  int           `json:"passed"`
	Failed  int           `json:"failed"`
	Errored int           `json:"errored"`
	Skipped int           `json:"skipped"`
	Runs    []TestRun     `json:"runs"`
	Files   []TestFile    `json:"files"`
	Left    []LeftInState `json:"left_in_state"`
}

type Report struct {
	Tool               string              `json:"tool"`
	ToolVersion        string              `json:"tool_version"`
	Operation          string              `json:"operation"`
	Summary            *Summary            `json:"summary"`
	Changes            []Change            `json:"changes"`
	Omitted            int                 `json:"resources_omitted"`
	NoopHidden         int                 `json:"noop_hidden"`
	Lists              []ListBlock         `json:"lists"`
	Found              []FoundResource     `json:"found"`
	FoundOmitted       int                 `json:"found_omitted"`
	Drift              []Change            `json:"drift"`
	Deferred           []Deferred          `json:"deferred"`
	Actions            []ActionInvocation  `json:"actions"`
	Applied            Applied             `json:"applied"`
	ActionsCompleted   int                 `json:"actions_completed"`
	Failures           []Failure           `json:"failures"`
	Policies           []PolicyResult      `json:"policies"`
	Errors             int                 `json:"errors"`
	Warnings           int                 `json:"warnings"`
	Diagnostics        []tfdiag.Diagnostic `json:"diagnostics"`
	DiagnosticsOmitted int                 `json:"diagnostics_omitted"`
	Outputs            []Output            `json:"outputs"`
	Providers          int                 `json:"providers_installed"`
	Tests              *Tests              `json:"tests"`
	Other              map[string]int      `json:"other"`
	otherMessages      []string
}

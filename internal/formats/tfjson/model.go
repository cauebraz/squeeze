package tfjson

import (
	"encoding/json"

	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type change struct {
	Address         string `json:"address"`
	PreviousAddress string `json:"previous_address,omitempty"`
	Action          string `json:"action"`
	Reason          string `json:"reason,omitempty"`
	ImportID        string `json:"import_id,omitempty"`
}

type deferred struct {
	Address string `json:"address"`
	Action  string `json:"action"`
	Reason  string `json:"reason"`
}

type listBlock struct {
	Address      string `json:"address"`
	ResourceType string `json:"resource_type,omitempty"`
	Total        int    `json:"total"`
	Complete     bool   `json:"complete"`
}

type foundResource struct {
	Address     string                     `json:"address"`
	DisplayName string                     `json:"display_name"`
	Identity    map[string]json.RawMessage `json:"identity,omitempty"`
}

type failure struct {
	Type    string `json:"type"`
	Address string `json:"address,omitempty"`
	Message string `json:"message"`
}

type policyResult struct {
	Policy      string  `json:"policy"`
	Target      string  `json:"target,omitempty"`
	Result      string  `json:"result"`
	Outcome     outcome `json:"outcome"`
	Enforcement string  `json:"enforcement,omitempty"`
}

type timing struct {
	Address string  `json:"address"`
	Seconds float64 `json:"seconds"`
}

type applied struct {
	Count   int      `json:"count"`
	Slowest []timing `json:"slowest"`
}

type output struct {
	Name      string          `json:"name"`
	Action    string          `json:"action,omitempty"`
	Sensitive bool            `json:"sensitive,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
}

type summary struct {
	Operation        string `json:"operation"`
	Add              int    `json:"add"`
	Change           int    `json:"change"`
	Remove           int    `json:"remove"`
	Import           int    `json:"import"`
	ActionInvocation int    `json:"action_invocation"`
	ActionFail       int    `json:"action_fail"`
}

type testRun struct {
	File        string              `json:"file"`
	Run         string              `json:"run"`
	Status      string              `json:"status"`
	ElapsedMs   *int64              `json:"elapsed_ms,omitempty"`
	Diagnostics []tfdiag.Diagnostic `json:"diagnostics"`
}

type testFile struct {
	File        string              `json:"file"`
	Status      string              `json:"status"`
	Diagnostics []tfdiag.Diagnostic `json:"diagnostics"`
}

type leftResource struct {
	Address    string `json:"address"`
	DeposedKey string `json:"deposed_key,omitempty"`
}

type leftInState struct {
	File      string         `json:"file"`
	Run       string         `json:"run,omitempty"`
	Reason    leftReason     `json:"reason"`
	State     string         `json:"state,omitempty"`
	Resources []leftResource `json:"resources"`
}

type tests struct {
	Status  string        `json:"status"`
	Passed  int           `json:"passed"`
	Failed  int           `json:"failed"`
	Errored int           `json:"errored"`
	Skipped int           `json:"skipped"`
	Runs    []testRun     `json:"runs"`
	Files   []testFile    `json:"files"`
	Left    []leftInState `json:"left_in_state"`
}

type report struct {
	Tool               string                    `json:"tool"`
	ToolVersion        string                    `json:"tool_version"`
	Operation          string                    `json:"operation"`
	Summary            *summary                  `json:"summary"`
	Changes            []change                  `json:"changes"`
	Omitted            int                       `json:"resources_omitted"`
	NoopHidden         int                       `json:"noop_hidden"`
	Lists              []listBlock               `json:"lists"`
	Found              []foundResource           `json:"found"`
	FoundOmitted       int                       `json:"found_omitted"`
	Drift              []change                  `json:"drift"`
	DriftOmitted       int                       `json:"drift_omitted"`
	Deferred           []deferred                `json:"deferred"`
	DeferredOmitted    int                       `json:"deferred_omitted"`
	Actions            []tfdiag.ActionInvocation `json:"actions"`
	Applied            applied                   `json:"applied"`
	ActionsCompleted   int                       `json:"actions_completed"`
	Failures           []failure                 `json:"failures"`
	Policies           []policyResult            `json:"policies"`
	Errors             int                       `json:"errors"`
	Warnings           int                       `json:"warnings"`
	Diagnostics        []tfdiag.Diagnostic       `json:"diagnostics"`
	DiagnosticsOmitted int                       `json:"diagnostics_omitted"`
	Outputs            []output                  `json:"outputs"`
	Providers          int                       `json:"providers_installed"`
	Tests              *tests                    `json:"tests"`
	Other              map[string]int            `json:"other"`
	otherMessages      []string
}

package tfjson

import (
	"bytes"
	"encoding/json"

	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type rawHeader struct {
	Level      string `json:"@level"`
	Message    string `json:"@message"`
	Type       string `json:"type"`
	AtTestFile string `json:"@testfile"`
	AtTestRun  string `json:"@testrun"`
	Terraform  string `json:"terraform"`
	Tofu       string `json:"tofu"`
}

type rawPayload struct {
	Diagnostic  *tfdiag.Raw          `json:"diagnostic"`
	Change      *rawChange           `json:"change"`
	Changes     *rawSummary          `json:"changes"`
	Hook        *rawHook             `json:"hook"`
	Outputs     map[string]rawOutput `json:"outputs"`
	Deferred    *rawDeferred         `json:"deferred_change"`
	TestRun     *rawTestRun          `json:"test_run"`
	TestFile    *rawTestFile         `json:"test_file"`
	TestSummary *rawTestSummary      `json:"test_summary"`
	Cleanup     *rawTestCleanup      `json:"test_cleanup"`
	Interrupt   *rawTestInterrupt    `json:"test_interrupt"`
	Invocation  *rawInvocation       `json:"invocation"`
	ListStart   *rawList             `json:"list_start"`
	ListFound   *rawListFound        `json:"list_resource_found"`
	ListDone    *rawList             `json:"list_complete"`
	Result      string               `json:"result"`
	Policy      string               `json:"policy_address"`
	Target      string               `json:"target_address"`
	PolicyMeta  *rawPolicyMeta       `json:"policy_metadata"`
	PolicyDiag  *tfdiag.Raw          `json:"policy_diagnostic"`
	PolicyInfo  *rawPolicyInfo       `json:"policy_info"`
	ListBlock   string               `json:"list_block_address"`
	Query       []rawQueryResult     `json:"results"`
}

type rawPolicyMeta struct {
	PolicyName       string `json:"policy_name"`
	EnforcementLevel string `json:"enforcement_level"`
}

type rawPolicyInfo struct {
	Message string `json:"message"`
}

type rawQueryResult struct {
	Target   string           `json:"target_address"`
	Result   string           `json:"result"`
	Policies []rawQueryPolicy `json:"policies"`
}

type rawQueryPolicy struct {
	Meta        rawPolicyMeta `json:"policy_metadata"`
	Diagnostics []tfdiag.Raw  `json:"diagnostics"`
	Result      string        `json:"result"`
}

type rawResource struct {
	Addr string `json:"addr"`
}

type rawActionAddr struct {
	Addr string `json:"addr"`
}

type rawList struct {
	Address      string `json:"address"`
	ResourceType string `json:"resource_type"`
	Total        int    `json:"total"`
}

type rawListFound struct {
	Address     string                     `json:"address"`
	DisplayName string                     `json:"display_name"`
	Identity    map[string]json.RawMessage `json:"identity"`
}

type rawInvocation struct {
	Action    rawActionAddr `json:"action_addr"`
	Lifecycle *struct {
		Resource rawResource `json:"triggering_resource"`
		Event    string      `json:"triggering_event"`
	} `json:"lifecycle_trigger"`
	Invoke *struct{} `json:"action_trigger"`
}

type rawChange struct {
	Resource         rawResource  `json:"resource"`
	PreviousResource *rawResource `json:"previous_resource"`
	Action           string       `json:"action"`
	Reason           string       `json:"reason"`
	Importing        *struct {
		ID string `json:"id"`
	} `json:"importing"`
}

type rawSummary struct {
	Add              int    `json:"add"`
	Change           int    `json:"change"`
	Import           int    `json:"import"`
	Remove           int    `json:"remove"`
	ActionInvocation int    `json:"action_invocation"`
	ActionFail       int    `json:"action_fail"`
	Operation        string `json:"operation"`
}

type rawHook struct {
	Resource rawResource   `json:"resource"`
	Action   rawHookAction `json:"action"`
	Elapsed  float64       `json:"elapsed_seconds"`
}

type rawHookAction struct {
	Name string
	Addr string
}

func (a *rawHookAction) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return nil
	}
	switch b[0] {
	case '"':
		return json.Unmarshal(b, &a.Name)
	case '{':
		var v rawActionAddr
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		a.Addr = v.Addr
	}
	return nil
}

type rawOutput struct {
	Sensitive bool            `json:"sensitive"`
	Value     json.RawMessage `json:"value"`
	Action    string          `json:"action"`
}

type rawDeferred struct {
	Reason string     `json:"reason"`
	Change *rawChange `json:"change"`
}

type rawTestRun struct {
	Path     string `json:"path"`
	Run      string `json:"run"`
	Progress string `json:"progress"`
	Status   string `json:"status"`
	Elapsed  *int64 `json:"elapsed"`
}

type rawTestFile struct {
	Path     string `json:"path"`
	Progress string `json:"progress"`
	Status   string `json:"status"`
}

type rawTestSummary struct {
	Status  string `json:"status"`
	Passed  int    `json:"passed"`
	Failed  int    `json:"failed"`
	Errored int    `json:"errored"`
	Skipped int    `json:"skipped"`
}

type rawTestResource struct {
	Instance   string `json:"instance"`
	DeposedKey string `json:"deposed_key"`
}

type rawTestCleanup struct {
	FailedResources  []rawTestResource `json:"failed_resources"`
	SkippedResources []rawTestResource `json:"skipped_resources"`
}

type rawTestInterrupt struct {
	State   []rawTestResource            `json:"state"`
	States  map[string][]rawTestResource `json:"states"`
	Planned []string                     `json:"planned"`
}

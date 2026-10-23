package tfplan

import (
	"encoding/json"

	"github.com/cauebraz/squeeze/internal/logscan"
)

type rawPlan struct {
	FormatVersion      string                `json:"format_version"`
	TerraformVersion   string                `json:"terraform_version"`
	PlannedValues      json.RawMessage       `json:"planned_values"`
	Values             json.RawMessage       `json:"values"`
	ResourceDrift      []rawResource         `json:"resource_drift"`
	ResourceChanges    []rawResource         `json:"resource_changes"`
	DeferredChanges    []rawDeferred         `json:"deferred_changes"`
	DeferredActions    []rawDeferredAction   `json:"deferred_action_invocations"`
	OutputChanges      map[string]rawChange  `json:"output_changes"`
	ActionInvocations  []rawAction           `json:"action_invocations"`
	RelevantAttributes []rawRelevantAttr     `json:"relevant_attributes"`
	Checks             []rawCheck            `json:"checks"`
	Errored            bool                  `json:"errored"`
	Complete           *bool                 `json:"complete"`
	Trailing           []logscan.FlaggedLine `json:"-"`
}

type rawCheck struct {
	Address   rawCheckAddress    `json:"address"`
	Status    string             `json:"status"`
	Instances []rawCheckInstance `json:"instances"`
}

type rawCheckAddress struct {
	Kind      string `json:"kind"`
	ToDisplay string `json:"to_display"`
}

type rawCheckInstance struct {
	Address  rawCheckAddress   `json:"address"`
	Status   string            `json:"status"`
	Problems []rawCheckProblem `json:"problems"`
}

type rawCheckProblem struct {
	Message string `json:"message"`
}

type rawResource struct {
	Address         string    `json:"address"`
	PreviousAddress string    `json:"previous_address"`
	Deposed         string    `json:"deposed"`
	ActionReason    string    `json:"action_reason"`
	Change          rawChange `json:"change"`
}

type rawChange struct {
	Actions         []string      `json:"actions"`
	Before          any           `json:"before"`
	After           any           `json:"after"`
	AfterUnknown    any           `json:"after_unknown"`
	BeforeSensitive any           `json:"before_sensitive"`
	AfterSensitive  any           `json:"after_sensitive"`
	ReplacePaths    [][]any       `json:"replace_paths"`
	Importing       *rawImporting `json:"importing"`
	GeneratedConfig string        `json:"generated_config"`
}

type rawImporting struct {
	ID       string `json:"id"`
	Identity any    `json:"identity"`
}

type rawDeferred struct {
	Reason         string      `json:"reason"`
	ResourceChange rawResource `json:"resource_change"`
}

type rawDeferredAction struct {
	Reason           string    `json:"reason"`
	ActionInvocation rawAction `json:"action_invocation"`
}

type rawAction struct {
	Address                string                  `json:"address"`
	LifecycleActionTrigger *rawLifecycleTrigger    `json:"lifecycle_action_trigger"`
	InvokeActionTrigger    *rawInvokeActionTrigger `json:"invoke_action_trigger"`
}

type rawLifecycleTrigger struct {
	TriggeringResourceAddress string `json:"triggering_resource_address"`
	ActionTriggerEvent        string `json:"action_trigger_event"`
}

type rawInvokeActionTrigger struct {
	CallingResourceAddress string `json:"calling_resource_address"`
}

type rawRelevantAttr struct {
	Resource  string `json:"resource"`
	Attribute []any  `json:"attribute"`
}

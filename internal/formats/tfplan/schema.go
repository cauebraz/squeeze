package tfplan

import "encoding/json"

type Plan struct {
	FormatVersion      string                `json:"format_version"`
	TerraformVersion   string                `json:"terraform_version"`
	PlannedValues      json.RawMessage       `json:"planned_values"`
	Values             json.RawMessage       `json:"values"`
	ResourceDrift      []PlanResource        `json:"resource_drift"`
	ResourceChanges    []PlanResource        `json:"resource_changes"`
	DeferredChanges    []PlanDeferred        `json:"deferred_changes"`
	DeferredActions    []PlanDeferredAction  `json:"deferred_action_invocations"`
	OutputChanges      map[string]PlanChange `json:"output_changes"`
	ActionInvocations  []PlanAction          `json:"action_invocations"`
	RelevantAttributes []PlanRelevantAttr    `json:"relevant_attributes"`
	Checks             []PlanCheck           `json:"checks"`
	Errored            bool                  `json:"errored"`
	Complete           *bool                 `json:"complete"`
}

type PlanCheck struct {
	Address   PlanCheckAddress    `json:"address"`
	Status    string              `json:"status"`
	Instances []PlanCheckInstance `json:"instances"`
}

type PlanCheckAddress struct {
	Kind      string `json:"kind"`
	ToDisplay string `json:"to_display"`
}

type PlanCheckInstance struct {
	Address  PlanCheckAddress   `json:"address"`
	Status   string             `json:"status"`
	Problems []PlanCheckProblem `json:"problems"`
}

type PlanCheckProblem struct {
	Message string `json:"message"`
}

type PlanResource struct {
	Address         string     `json:"address"`
	PreviousAddress string     `json:"previous_address"`
	Deposed         string     `json:"deposed"`
	ActionReason    string     `json:"action_reason"`
	Change          PlanChange `json:"change"`
}

type PlanChange struct {
	Actions         []string       `json:"actions"`
	Before          any            `json:"before"`
	After           any            `json:"after"`
	AfterUnknown    any            `json:"after_unknown"`
	BeforeSensitive any            `json:"before_sensitive"`
	AfterSensitive  any            `json:"after_sensitive"`
	ReplacePaths    [][]any        `json:"replace_paths"`
	Importing       *PlanImporting `json:"importing"`
	GeneratedConfig string         `json:"generated_config"`
}

type PlanImporting struct {
	ID       string `json:"id"`
	Identity any    `json:"identity"`
}

type PlanDeferred struct {
	Reason         string       `json:"reason"`
	ResourceChange PlanResource `json:"resource_change"`
}

type PlanDeferredAction struct {
	Reason           string     `json:"reason"`
	ActionInvocation PlanAction `json:"action_invocation"`
}

type PlanAction struct {
	Address                string                   `json:"address"`
	LifecycleActionTrigger *PlanLifecycleTrigger    `json:"lifecycle_action_trigger"`
	InvokeActionTrigger    *PlanInvokeActionTrigger `json:"invoke_action_trigger"`
}

type PlanLifecycleTrigger struct {
	TriggeringResourceAddress string `json:"triggering_resource_address"`
	ActionTriggerEvent        string `json:"action_trigger_event"`
}

type PlanInvokeActionTrigger struct {
	CallingResourceAddress string `json:"calling_resource_address"`
}

type PlanRelevantAttr struct {
	Resource  string `json:"resource"`
	Attribute []any  `json:"attribute"`
}

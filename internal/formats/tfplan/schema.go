package tfplan

import "encoding/json"

type Plan struct {
	FormatVersion    string                `json:"format_version"`
	TerraformVersion string                `json:"terraform_version"`
	PlannedValues    json.RawMessage       `json:"planned_values"`
	Values           json.RawMessage       `json:"values"`
	ResourceDrift    []PlanResource        `json:"resource_drift"`
	ResourceChanges  []PlanResource        `json:"resource_changes"`
	OutputChanges    map[string]PlanChange `json:"output_changes"`
	Errored          bool                  `json:"errored"`
	Complete         *bool                 `json:"complete"`
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
}

type PlanImporting struct {
	ID       string `json:"id"`
	Identity any    `json:"identity"`
}

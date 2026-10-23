package tfplan

import (
	"encoding/json"
	"fmt"

	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type Action int

const (
	Noop Action = iota
	Create
	Read
	Update
	Delete
	Replace
	ReplaceCBD
	Forget
	ForgetThenCreate
	CreateThenForget
	Import
)

var allActions = []Action{Noop, Create, Read, Update, Delete, Replace, ReplaceCBD, Forget, ForgetThenCreate, CreateThenForget, Import}

var actionNames = [...]string{
	Noop:             "no-op",
	Create:           "create",
	Read:             "read",
	Update:           "update",
	Delete:           "delete",
	Replace:          "replace",
	ReplaceCBD:       "replace-create-before-destroy",
	Forget:           "forget",
	ForgetThenCreate: "forget-then-create",
	CreateThenForget: "create-then-forget",
	Import:           "import",
}

var actionSymbols = [...]string{
	Noop:             "=",
	Create:           "+",
	Read:             "<=",
	Update:           "~",
	Delete:           "-",
	Replace:          "-/+",
	ReplaceCBD:       "+/-",
	Forget:           ".",
	ForgetThenCreate: "./+",
	CreateThenForget: "+/.",
	Import:           ">>",
}

var actionRanks = [...]int{
	Delete:           0,
	Replace:          1,
	ReplaceCBD:       2,
	ForgetThenCreate: 3,
	CreateThenForget: 4,
	Forget:           5,
	Update:           6,
	Create:           7,
	Read:             8,
	Import:           9,
	Noop:             10,
}

func (a Action) valid() bool {
	return a >= Noop && a <= Import
}

func (a Action) String() string {
	if !a.valid() {
		return fmt.Sprintf("action(%d)", int(a))
	}
	return actionNames[a]
}

func (a Action) Symbol() string {
	if !a.valid() {
		return "?"
	}
	return actionSymbols[a]
}

func (a Action) Rank() int {
	if !a.valid() {
		return len(actionRanks)
	}
	return actionRanks[a]
}

func (a Action) IsReplace() bool {
	return a == Replace || a == ReplaceCBD || a == ForgetThenCreate || a == CreateThenForget
}

func (a Action) MarshalText() ([]byte, error) {
	if !a.valid() {
		return nil, fmt.Errorf("invalid action %d", int(a))
	}
	return []byte(actionNames[a]), nil
}

type AttrKind int

const (
	Modified AttrKind = iota
	Added
	Removed
)

var attrKindNames = [...]string{Modified: "update", Added: "add", Removed: "remove"}

var attrKindSymbols = [...]string{Modified: "~", Added: "+", Removed: "-"}

func (k AttrKind) valid() bool {
	return k >= Modified && k <= Removed
}

func (k AttrKind) MarshalText() ([]byte, error) {
	if !k.valid() {
		return nil, fmt.Errorf("invalid attribute kind %d", int(k))
	}
	return []byte(attrKindNames[k]), nil
}

func (k AttrKind) Symbol() string {
	if !k.valid() {
		return "?"
	}
	return attrKindSymbols[k]
}

type Scalar struct {
	Value any
}

func (s Scalar) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Value)
}

type Summary struct {
	Import          int  `json:"import"`
	Add             int  `json:"add"`
	Change          int  `json:"change"`
	Destroy         int  `json:"destroy"`
	Replace         int  `json:"replace"`
	Forget          int  `json:"forget"`
	Move            int  `json:"move"`
	Read            int  `json:"read"`
	Noop            int  `json:"noop"`
	Drift           int  `json:"drift"`
	DriftIgnored    int  `json:"drift_ignored"`
	Deferred        int  `json:"deferred"`
	DeferredActions int  `json:"deferred_actions"`
	Actions         int  `json:"actions"`
	Outputs         int  `json:"outputs"`
	ChecksFailed    int  `json:"checks_failed"`
	Errored         bool `json:"errored"`
	Complete        bool `json:"complete"`
}

func (s Summary) Changed() bool {
	return s.Import+s.Add+s.Change+s.Destroy+s.Forget+s.Move+s.Read > 0
}

type AttrChange struct {
	Path              string   `json:"path"`
	Kind              AttrKind `json:"kind"`
	Before            *Scalar  `json:"before,omitempty"`
	After             *Scalar  `json:"after,omitempty"`
	Unknown           bool     `json:"unknown,omitempty"`
	Sensitive         bool     `json:"sensitive,omitempty"`
	ForcesReplacement bool     `json:"forces_replacement,omitempty"`
	Reordered         bool     `json:"reordered,omitempty"`
	at                path
	beforeAt          path
}

type ResourceChange struct {
	Address         string       `json:"address"`
	PreviousAddress string       `json:"previous_address,omitempty"`
	Deposed         string       `json:"deposed,omitempty"`
	Action          Action       `json:"action"`
	Importing       bool         `json:"importing,omitempty"`
	Reason          string       `json:"reason,omitempty"`
	Irrelevant      bool         `json:"irrelevant,omitempty"`
	GeneratedLines  int          `json:"generated_config_lines,omitempty"`
	GeneratedConfig string       `json:"generated_config,omitempty"`
	ReplacedBy      []string     `json:"replaced_by,omitempty"`
	Attrs           []AttrChange `json:"attrs,omitempty"`
	AttrsOmitted    int          `json:"attrs_omitted,omitempty"`
}

type DeferredChange struct {
	Address string `json:"address"`
	Action  Action `json:"action"`
	Reason  string `json:"reason"`
}

type ActionInvocation struct {
	Address  string `json:"address"`
	Event    string `json:"event,omitempty"`
	Resource string `json:"resource,omitempty"`
	Invoked  bool   `json:"invoked,omitempty"`
}

type DeferredAction struct {
	ActionInvocation
	Reason string `json:"reason"`
}

type OutputChange struct {
	Name      string `json:"name"`
	Action    Action `json:"action"`
	Unknown   bool   `json:"unknown,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
}

type CheckResult struct {
	Address  string   `json:"address"`
	Kind     string   `json:"kind"`
	Status   string   `json:"status"`
	Problems []string `json:"problems,omitempty"`
}

type Report struct {
	Tool            string              `json:"tool"`
	ToolVersion     string              `json:"tool_version"`
	Summary         Summary             `json:"summary"`
	Changes         []ResourceChange    `json:"changes"`
	Omitted         int                 `json:"resources_omitted"`
	NoopHidden      int                 `json:"noop_hidden"`
	Drift           []ResourceChange    `json:"drift"`
	DriftOmitted    int                 `json:"drift_omitted"`
	Deferred        []DeferredChange    `json:"deferred"`
	DeferredActions []DeferredAction    `json:"deferred_actions"`
	Actions         []ActionInvocation  `json:"actions"`
	Outputs         []OutputChange      `json:"outputs"`
	Checks          []CheckResult       `json:"checks"`
	Diagnostics     []tfdiag.Diagnostic `json:"diagnostics,omitempty"`
}

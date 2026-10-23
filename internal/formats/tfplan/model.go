package tfplan

import (
	"encoding/json"
	"fmt"

	"github.com/cauebraz/squeeze/internal/tfdiag"
)

type action int

const (
	actionNoop action = iota
	actionCreate
	actionRead
	actionUpdate
	actionDelete
	actionReplace
	actionReplaceCBD
	actionForget
	actionForgetThenCreate
	actionCreateThenForget
	actionImport
	actionUnknown
)

var allActions = []action{actionNoop, actionCreate, actionRead, actionUpdate, actionDelete, actionReplace, actionReplaceCBD, actionForget, actionForgetThenCreate, actionCreateThenForget, actionImport, actionUnknown}

var actionNames = [...]string{
	actionNoop:             "no-op",
	actionCreate:           "create",
	actionRead:             "read",
	actionUpdate:           "update",
	actionDelete:           "delete",
	actionReplace:          "replace",
	actionReplaceCBD:       "replace-create-before-destroy",
	actionForget:           "forget",
	actionForgetThenCreate: "forget-then-create",
	actionCreateThenForget: "create-then-forget",
	actionImport:           "import",
	actionUnknown:          "unknown",
}

var actionSymbols = [...]string{
	actionNoop:             "=",
	actionCreate:           "+",
	actionRead:             "<=",
	actionUpdate:           "~",
	actionDelete:           "-",
	actionReplace:          "-/+",
	actionReplaceCBD:       "+/-",
	actionForget:           ".",
	actionForgetThenCreate: "./+",
	actionCreateThenForget: "+/.",
	actionImport:           ">>",
	actionUnknown:          "?",
}

var actionRanks = [...]int{
	actionUnknown:          0,
	actionDelete:           1,
	actionReplace:          2,
	actionReplaceCBD:       3,
	actionForgetThenCreate: 4,
	actionCreateThenForget: 5,
	actionForget:           6,
	actionUpdate:           7,
	actionCreate:           8,
	actionRead:             9,
	actionImport:           10,
	actionNoop:             11,
}

func (a action) valid() bool {
	return a >= actionNoop && a <= actionUnknown
}

func (a action) String() string {
	if !a.valid() {
		return fmt.Sprintf("action(%d)", int(a))
	}
	return actionNames[a]
}

func (a action) symbol() string {
	if !a.valid() {
		return "?"
	}
	return actionSymbols[a]
}

func (a action) rank() int {
	if !a.valid() {
		return len(actionRanks)
	}
	return actionRanks[a]
}

func (a action) isReplace() bool {
	return a == actionReplace || a == actionReplaceCBD || a == actionForgetThenCreate || a == actionCreateThenForget
}

func (a action) MarshalText() ([]byte, error) {
	if !a.valid() {
		return nil, fmt.Errorf("invalid action %d", int(a))
	}
	return []byte(actionNames[a]), nil
}

type attrKind int

const (
	attrModified attrKind = iota
	attrAdded
	attrRemoved
)

var attrKindNames = [...]string{attrModified: "update", attrAdded: "add", attrRemoved: "remove"}

var attrKindSymbols = [...]string{attrModified: "~", attrAdded: "+", attrRemoved: "-"}

func (k attrKind) valid() bool {
	return k >= attrModified && k <= attrRemoved
}

func (k attrKind) MarshalText() ([]byte, error) {
	if !k.valid() {
		return nil, fmt.Errorf("invalid attribute kind %d", int(k))
	}
	return []byte(attrKindNames[k]), nil
}

func (k attrKind) symbol() string {
	if !k.valid() {
		return "?"
	}
	return attrKindSymbols[k]
}

type scalar struct {
	Value any
}

func (s scalar) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.Value)
}

type summary struct {
	Import          int  `json:"import"`
	Add             int  `json:"add"`
	Change          int  `json:"change"`
	Destroy         int  `json:"destroy"`
	Replace         int  `json:"replace"`
	Forget          int  `json:"forget"`
	Move            int  `json:"move"`
	Read            int  `json:"read"`
	Noop            int  `json:"noop"`
	Unknown         int  `json:"unknown"`
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

func (s summary) changed() bool {
	return s.Import+s.Add+s.Change+s.Destroy+s.Forget+s.Move+s.Read+s.Unknown > 0
}

type attrChange struct {
	Path              string   `json:"path"`
	Kind              attrKind `json:"kind"`
	Before            *scalar  `json:"before,omitempty"`
	After             *scalar  `json:"after,omitempty"`
	Unknown           bool     `json:"unknown,omitempty"`
	Sensitive         bool     `json:"sensitive,omitempty"`
	ForcesReplacement bool     `json:"forces_replacement,omitempty"`
	Reordered         bool     `json:"reordered,omitempty"`
	at                path
	beforeAt          path
}

type resourceChange struct {
	Address         string       `json:"address"`
	PreviousAddress string       `json:"previous_address,omitempty"`
	Deposed         string       `json:"deposed,omitempty"`
	Action          action       `json:"action"`
	Actions         []string     `json:"actions,omitempty"`
	Importing       bool         `json:"importing,omitempty"`
	Reason          string       `json:"reason,omitempty"`
	Irrelevant      bool         `json:"irrelevant,omitempty"`
	GeneratedLines  int          `json:"generated_config_lines,omitempty"`
	GeneratedConfig string       `json:"generated_config,omitempty"`
	ReplacedBy      []string     `json:"replaced_by,omitempty"`
	Attrs           []attrChange `json:"attrs,omitempty"`
	AttrsOmitted    int          `json:"attrs_omitted,omitempty"`
}

type deferredChange struct {
	Address string   `json:"address"`
	Action  action   `json:"action"`
	Actions []string `json:"actions,omitempty"`
	Reason  string   `json:"reason"`
}

type deferredAction struct {
	tfdiag.ActionInvocation
	Reason string `json:"reason"`
}

type outputChange struct {
	Name      string   `json:"name"`
	Action    action   `json:"action"`
	Actions   []string `json:"actions,omitempty"`
	Unknown   bool     `json:"unknown,omitempty"`
	Sensitive bool     `json:"sensitive,omitempty"`
}

type checkResult struct {
	Address  string   `json:"address"`
	Kind     string   `json:"kind"`
	Status   string   `json:"status"`
	Problems []string `json:"problems,omitempty"`
}

type report struct {
	Tool            string                    `json:"tool"`
	ToolVersion     string                    `json:"tool_version"`
	Summary         summary                   `json:"summary"`
	Changes         []resourceChange          `json:"changes"`
	Omitted         int                       `json:"resources_omitted"`
	NoopHidden      int                       `json:"noop_hidden"`
	Drift           []resourceChange          `json:"drift"`
	DriftOmitted    int                       `json:"drift_omitted"`
	DriftHidden     int                       `json:"drift_hidden"`
	Deferred        []deferredChange          `json:"deferred"`
	DeferredOmitted int                       `json:"deferred_omitted"`
	DeferredActions []deferredAction          `json:"deferred_actions"`
	Actions         []tfdiag.ActionInvocation `json:"actions"`
	Outputs         []outputChange            `json:"outputs"`
	Checks          []checkResult             `json:"checks"`
	Diagnostics     []tfdiag.Diagnostic       `json:"diagnostics,omitempty"`
}

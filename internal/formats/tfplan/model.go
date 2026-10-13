package tfplan

import (
	"fmt"
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
	Import
	Forget
)

var actionNames = [...]string{
	Noop:       "no-op",
	Create:     "create",
	Read:       "read",
	Update:     "update",
	Delete:     "delete",
	Replace:    "replace",
	ReplaceCBD: "replace-create-before-destroy",
	Import:     "import",
	Forget:     "forget",
}

var actionSymbols = [...]string{
	Noop:       "=",
	Create:     "+",
	Read:       "<=",
	Update:     "~",
	Delete:     "-",
	Replace:    "-/+",
	ReplaceCBD: "+/-",
	Import:     ">>",
	Forget:     "-",
}

var actionRanks = [...]int{
	Delete:     0,
	Replace:    1,
	ReplaceCBD: 2,
	Forget:     3,
	Update:     4,
	Create:     5,
	Read:       6,
	Import:     7,
	Noop:       8,
}

func (a Action) valid() bool {
	return a >= Noop && a <= Forget
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

func (a Action) MarshalText() ([]byte, error) {
	if !a.valid() {
		return nil, fmt.Errorf("tfplan: invalid action %d", int(a))
	}
	return []byte(actionNames[a]), nil
}

type Summary struct {
	Add      int  `json:"add"`
	Change   int  `json:"change"`
	Destroy  int  `json:"destroy"`
	Replace  int  `json:"replace"`
	Read     int  `json:"read"`
	Import   int  `json:"import"`
	Forget   int  `json:"forget"`
	Move     int  `json:"move"`
	Noop     int  `json:"noop"`
	Drift    int  `json:"drift"`
	Outputs  int  `json:"outputs"`
	Errored  bool `json:"errored"`
	Complete bool `json:"complete"`
}

func (s Summary) Changed() bool {
	return s.Add+s.Change+s.Destroy+s.Replace+s.Read+s.Import+s.Forget+s.Move > 0
}

type AttrChange struct {
	Path              string `json:"path"`
	Before            string `json:"before,omitempty"`
	After             string `json:"after,omitempty"`
	Unknown           bool   `json:"unknown,omitempty"`
	Sensitive         bool   `json:"sensitive,omitempty"`
	ForcesReplacement bool   `json:"forces_replacement,omitempty"`
}

type ResourceChange struct {
	Address         string       `json:"address"`
	PreviousAddress string       `json:"previous_address,omitempty"`
	Deposed         string       `json:"deposed,omitempty"`
	Action          Action       `json:"action"`
	Importing       bool         `json:"importing,omitempty"`
	Reason          string       `json:"reason,omitempty"`
	ReplacedBy      []string     `json:"replaced_by,omitempty"`
	Attrs           []AttrChange `json:"attrs,omitempty"`
	AttrsOmitted    int          `json:"attrs_omitted,omitempty"`
}

type OutputChange struct {
	Name      string `json:"name"`
	Action    Action `json:"action"`
	Unknown   bool   `json:"unknown,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
}

type Report struct {
	Tool        string           `json:"tool"`
	ToolVersion string           `json:"tool_version"`
	Summary     Summary          `json:"summary"`
	Changes     []ResourceChange `json:"changes"`
	Omitted     int              `json:"omitted"`
	Drift       []ResourceChange `json:"drift"`
	Outputs     []OutputChange   `json:"outputs"`
}

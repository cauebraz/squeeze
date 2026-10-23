package pytest

import "github.com/cauebraz/squeeze/internal/core"

type summary struct {
	Failed     int            `json:"failed"`
	Passed     int            `json:"passed"`
	Skipped    int            `json:"skipped"`
	Deselected int            `json:"deselected"`
	XFailed    int            `json:"xfailed"`
	XPassed    int            `json:"xpassed"`
	Warnings   int            `json:"warnings"`
	Errors     int            `json:"errors"`
	Other      map[string]int `json:"other"`
	Duration   string         `json:"duration"`
	Line       string         `json:"line"`
}

type capturedSection struct {
	Name string `json:"name"`
	core.Block
}

type failureKind string

const (
	kindFailed failureKind = "failed"
	kindError  failureKind = "error"
)

func (k failureKind) word() string {
	switch k {
	case kindError:
		return "ERROR"
	case kindFailed:
	}
	return "FAILED"
}

type failure struct {
	Kind    failureKind `json:"kind"`
	Test    string      `json:"test"`
	Subtest string      `json:"subtest,omitempty"`
	When    string      `json:"when,omitempty"`
	Message string      `json:"message,omitempty"`
	core.Block
	Hidden            int               `json:"hidden,omitempty"`
	Captured          []capturedSection `json:"captured,omitempty"`
	FromSummaryOnly   bool              `json:"from_summary_only,omitempty"`
	Location          string            `json:"location,omitempty"`
	SameLocationTests []string          `json:"same_location_tests,omitempty"`
}

type warningMessage struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

type report struct {
	Tool              string           `json:"tool"`
	ToolVersion       string           `json:"tool_version"`
	SummaryFound      bool             `json:"summary_found"`
	Summary           summary          `json:"summary"`
	Interrupted       string           `json:"interrupted,omitempty"`
	InternalError     bool             `json:"internal_error,omitempty"`
	Stopped           string           `json:"stopped,omitempty"`
	Failures          []failure        `json:"failures"`
	TestsOmitted      int              `json:"tests_omitted"`
	FailuresFolded    int              `json:"failures_folded"`
	WarningMessages   []warningMessage `json:"warning_messages"`
	WarningsOmitted   int              `json:"warnings_omitted"`
	Notes             []string         `json:"notes"`
	NotesOmitted      int              `json:"notes_omitted"`
	Outcomes          []string         `json:"outcomes"`
	OtherSections     map[string]int   `json:"other_sections"`
	OtherSectionLines []sectionLines   `json:"other_section_lines"`
}

type sectionLines struct {
	Name  string   `json:"name"`
	Lines []string `json:"lines"`
}

package pytest

type Summary struct {
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

type Failure struct {
	Kind      string   `json:"kind"`
	Test      string   `json:"test"`
	When      string   `json:"when,omitempty"`
	Message   string   `json:"message,omitempty"`
	Lines     []string `json:"lines"`
	Hidden    int      `json:"hidden,omitempty"`
	Captured  []string `json:"captured,omitempty"`
	Omitted   int      `json:"omitted,omitempty"`
	Untracked bool     `json:"from_summary_only,omitempty"`
}

type Warning struct {
	Message string `json:"message"`
	Count   int    `json:"count"`
}

type Report struct {
	Tool            string         `json:"tool"`
	Version         string         `json:"version"`
	Complete        bool           `json:"complete"`
	Summary         Summary        `json:"summary"`
	Interrupted     string         `json:"interrupted,omitempty"`
	Stopped         string         `json:"stopped,omitempty"`
	Failures        []Failure      `json:"failures"`
	FailuresOmitted int            `json:"failures_omitted"`
	Warnings        []Warning      `json:"warnings"`
	WarningsOmitted int            `json:"warnings_omitted"`
	Notes           []string       `json:"notes"`
	NotesOmitted    int            `json:"notes_omitted"`
	Outcomes        []string       `json:"outcomes"`
	Sections        map[string]int `json:"other_sections"`
	sectionLines    map[string][]string
	sectionOrder    []string
}

package gotest

type Counts struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

type PackageCounts struct {
	OK          int `json:"ok"`
	Failed      int `json:"failed"`
	BuildFailed int `json:"build_failed"`
	NoTests     int `json:"no_tests"`
}

type Output struct {
	Lines   []string `json:"lines"`
	Logs    int      `json:"logs_hidden,omitempty"`
	Stack   int      `json:"stack_hidden,omitempty"`
	Omitted int      `json:"omitted,omitempty"`
}

type Failure struct {
	Package    string            `json:"package"`
	Test       string            `json:"test"`
	Elapsed    float64           `json:"elapsed,omitempty"`
	Unfinished bool              `json:"unfinished,omitempty"`
	Attrs      map[string]string `json:"attrs,omitempty"`
	Artifacts  string            `json:"artifacts,omitempty"`
	Output     Output            `json:"output"`
}

type BuildFailure struct {
	Package string `json:"package"`
	Output  Output `json:"output"`
}

type PackageFailure struct {
	Package    string `json:"package"`
	Unfinished bool   `json:"unfinished,omitempty"`
	Output     Output `json:"output"`
}

type Benchmark struct {
	Package string `json:"package"`
	Result  string `json:"result"`
}

type TestRef struct {
	Package string `json:"package"`
	Test    string `json:"test"`
}

type PackageRef struct {
	Package string  `json:"package"`
	Elapsed float64 `json:"elapsed,omitempty"`
}

type Report struct {
	Tool            string           `json:"tool"`
	Status          string           `json:"status"`
	Tests           Counts           `json:"tests"`
	Packages        PackageCounts    `json:"packages"`
	BuildFailures   []BuildFailure   `json:"build_failures"`
	Failures        []Failure        `json:"failures"`
	FailuresOmitted int              `json:"failures_omitted"`
	PackageFailures []PackageFailure `json:"package_failures"`
	Benchmarks      []Benchmark      `json:"benchmarks"`
	Skipped         []TestRef        `json:"skipped"`
	Passing         []PackageRef     `json:"passing_packages"`
	NoTests         []string         `json:"packages_without_tests"`
	Text            Output           `json:"text"`
	Panics          int              `json:"panics"`
	Timeouts        int              `json:"timeouts"`
	Races           int              `json:"races"`
	Other           map[string]int   `json:"other"`
}

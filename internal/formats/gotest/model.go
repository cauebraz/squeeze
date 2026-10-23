package gotest

import "github.com/cauebraz/squeeze/internal/core"

type status string

const (
	statusPass status = "pass"
	statusFail status = "fail"
)

type testCounts struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

type packageCounts struct {
	OK          int `json:"ok"`
	Failed      int `json:"failed"`
	BuildFailed int `json:"build_failed"`
	NoTests     int `json:"no_tests"`
}

type output struct {
	core.Block
	LogsHidden  int `json:"logs_hidden,omitempty"`
	StackHidden int `json:"stack_hidden,omitempty"`
}

type failure struct {
	Package    string            `json:"package"`
	Test       string            `json:"test"`
	Run        int               `json:"run,omitempty"`
	Runs       int               `json:"runs,omitempty"`
	Elapsed    float64           `json:"elapsed,omitempty"`
	Unfinished bool              `json:"unfinished,omitempty"`
	Attrs      map[string]string `json:"attrs,omitempty"`
	Artifacts  string            `json:"artifacts,omitempty"`
	Output     output            `json:"output"`
}

type buildFailure struct {
	Package string `json:"package"`
	Output  output `json:"output"`
}

type packageFailure struct {
	Package    string `json:"package"`
	Unfinished bool   `json:"unfinished,omitempty"`
	Output     output `json:"output"`
}

type benchmark struct {
	Package string `json:"package"`
	Result  string `json:"result"`
}

type testRef struct {
	Package string `json:"package"`
	Test    string `json:"test"`
}

type packageRef struct {
	Package string  `json:"package"`
	Elapsed float64 `json:"elapsed,omitempty"`
}

type packageCoverage struct {
	Package  string `json:"package"`
	Coverage string `json:"coverage"`
}

type packageSeed struct {
	Package string `json:"package"`
	Seed    string `json:"seed"`
	Failed  bool   `json:"failed,omitempty"`
}

type report struct {
	Tool                 string            `json:"tool"`
	ToolVersion          string            `json:"tool_version"`
	Status               status            `json:"status"`
	Tests                testCounts        `json:"tests"`
	Packages             packageCounts     `json:"packages"`
	BuildFailures        []buildFailure    `json:"build_failures"`
	Failures             []failure         `json:"failures"`
	TestsOmitted         int               `json:"tests_omitted"`
	PackageFailures      []packageFailure  `json:"package_failures"`
	Benchmarks           []benchmark       `json:"benchmarks"`
	Skipped              []testRef         `json:"skipped"`
	PassingPackages      []packageRef      `json:"passing_packages"`
	PackagesWithoutTests []string          `json:"packages_without_tests"`
	Coverage             []packageCoverage `json:"coverage"`
	CoverageOmitted      int               `json:"coverage_omitted"`
	Shuffle              []packageSeed     `json:"shuffle"`
	Text                 output            `json:"text"`
	WarningMessages      []string          `json:"warning_messages"`
	Panics               int               `json:"panics"`
	Timeouts             int               `json:"timeouts"`
	Races                int               `json:"races"`
	Other                map[string]int    `json:"other"`
	testsUnlisted        bool
}

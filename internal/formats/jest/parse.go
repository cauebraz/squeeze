package jest

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cauebraz/squeeze/internal/core"
)

var errNotJest = errors.New("input is not a jest --json or vitest json report")

const vitestNotice = "JSON report written to "

type rawLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type testStatus string

const (
	testFailed   testStatus = "failed"
	testPassed   testStatus = "passed"
	testPending  testStatus = "pending"
	testSkipped  testStatus = "skipped"
	testDisabled testStatus = "disabled"
	testFocused  testStatus = "focused"
	testTodo     testStatus = "todo"
)

type rawAssertion struct {
	AncestorTitles  []string     `json:"ancestorTitles"`
	Title           string       `json:"title"`
	FullName        string       `json:"fullName"`
	Status          testStatus   `json:"status"`
	FailureMessages []string     `json:"failureMessages"`
	Location        *rawLocation `json:"location"`
}

type rawTestFile struct {
	Name             string         `json:"name"`
	Status           testStatus     `json:"status"`
	Message          string         `json:"message"`
	AssertionResults []rawAssertion `json:"assertionResults"`
}

type rawError struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Stack   string `json:"stack"`
}

type rawSnapshot struct {
	Added        int  `json:"added"`
	Failure      bool `json:"failure"`
	FilesRemoved int  `json:"filesRemoved"`
	Total        int  `json:"total"`
	Unchecked    int  `json:"unchecked"`
	Unmatched    int  `json:"unmatched"`
	Updated      int  `json:"updated"`
}

type rawOutput struct {
	NumTotalTests             int           `json:"numTotalTests"`
	NumPassedTests            int           `json:"numPassedTests"`
	NumFailedTests            int           `json:"numFailedTests"`
	NumPendingTests           int           `json:"numPendingTests"`
	NumTodoTests              int           `json:"numTodoTests"`
	NumRuntimeErrorTestSuites *int          `json:"numRuntimeErrorTestSuites"`
	Success                   bool          `json:"success"`
	WasInterrupted            bool          `json:"wasInterrupted"`
	OpenHandles               []rawError    `json:"openHandles"`
	RunExecError              *rawError     `json:"runExecError"`
	Snapshot                  rawSnapshot   `json:"snapshot"`
	TestResults               []rawTestFile `json:"testResults"`
	Trailing                  string        `json:"-"`
}

func parse(input []byte) (*rawOutput, error) {
	var out rawOutput
	rest, err := core.DecodeJSON(input, &out)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if out.TestResults == nil {
		return nil, errNotJest
	}
	for _, f := range out.TestResults {
		if f.Name == "" {
			return nil, errNotJest
		}
	}
	rest = bytes.TrimSpace(rest)
	if bytes.HasPrefix(rest, []byte(vitestNotice)) && !bytes.Contains(rest, []byte("\n")) {
		rest = nil
	}
	out.Trailing = string(rest)
	return &out, nil
}

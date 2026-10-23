package jest

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/cauebraz/squeeze/internal/core"
)

var ErrNotJest = errors.New("input is not a jest --json or vitest json report")

const vitestNotice = "JSON report written to "

type Location struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type Assertion struct {
	AncestorTitles  []string  `json:"ancestorTitles"`
	Title           string    `json:"title"`
	FullName        string    `json:"fullName"`
	Status          string    `json:"status"`
	FailureMessages []string  `json:"failureMessages"`
	Location        *Location `json:"location"`
}

type TestFile struct {
	Name             string      `json:"name"`
	Status           string      `json:"status"`
	Message          string      `json:"message"`
	AssertionResults []Assertion `json:"assertionResults"`
}

type SerializedError struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Stack   string `json:"stack"`
}

type Snapshot struct {
	Added        int  `json:"added"`
	Failure      bool `json:"failure"`
	FilesRemoved int  `json:"filesRemoved"`
	Total        int  `json:"total"`
	Unchecked    int  `json:"unchecked"`
	Unmatched    int  `json:"unmatched"`
	Updated      int  `json:"updated"`
}

type Output struct {
	NumTotalTests             int               `json:"numTotalTests"`
	NumPassedTests            int               `json:"numPassedTests"`
	NumFailedTests            int               `json:"numFailedTests"`
	NumPendingTests           int               `json:"numPendingTests"`
	NumTodoTests              int               `json:"numTodoTests"`
	NumRuntimeErrorTestSuites *int              `json:"numRuntimeErrorTestSuites"`
	Success                   bool              `json:"success"`
	WasInterrupted            bool              `json:"wasInterrupted"`
	OpenHandles               []SerializedError `json:"openHandles"`
	RunExecError              *SerializedError  `json:"runExecError"`
	Snapshot                  Snapshot          `json:"snapshot"`
	TestResults               *[]TestFile       `json:"testResults"`
	Trailing                  string            `json:"-"`
}

func Parse(input []byte) (*Output, error) {
	var out Output
	rest, err := core.DecodeJSON(input, &out)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if out.TestResults == nil {
		return nil, ErrNotJest
	}
	for _, f := range *out.TestResults {
		if f.Name == "" {
			return nil, ErrNotJest
		}
	}
	rest = bytes.TrimSpace(rest)
	if bytes.HasPrefix(rest, []byte(vitestNotice)) && !bytes.Contains(rest, []byte("\n")) {
		rest = nil
	}
	out.Trailing = string(rest)
	return &out, nil
}

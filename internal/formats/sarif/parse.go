package sarif

import (
	"encoding/json"
	"errors"
	"fmt"
)

var ErrNotSARIF = errors.New("input is not a SARIF 2.1.0 log")

type Message struct {
	Text string `json:"text"`
}

type Region struct {
	StartLine   int `json:"startLine"`
	StartColumn int `json:"startColumn"`
}

type Location struct {
	PhysicalLocation *struct {
		ArtifactLocation *struct {
			URI string `json:"uri"`
		} `json:"artifactLocation"`
		Region *Region `json:"region"`
	} `json:"physicalLocation"`
}

type Suppression struct {
	Status string `json:"status"`
}

type RawResult struct {
	RuleID    string `json:"ruleId"`
	RuleIndex *int   `json:"ruleIndex"`
	Rule      *struct {
		ID string `json:"id"`
	} `json:"rule"`
	Kind         string        `json:"kind"`
	Level        string        `json:"level"`
	Message      Message       `json:"message"`
	Locations    []Location    `json:"locations"`
	Suppressions []Suppression `json:"suppressions"`
}

type Rule struct {
	ID                   string   `json:"id"`
	ShortDescription     *Message `json:"shortDescription"`
	DefaultConfiguration *struct {
		Level string `json:"level"`
	} `json:"defaultConfiguration"`
}

type Notification struct {
	Level     string     `json:"level"`
	Message   Message    `json:"message"`
	Locations []Location `json:"locations"`
}

type Invocation struct {
	ExecutionSuccessful        *bool          `json:"executionSuccessful"`
	ToolExecutionNotifications []Notification `json:"toolExecutionNotifications"`
	ToolConfigNotifications    []Notification `json:"toolConfigurationNotifications"`
}

type Run struct {
	Tool struct {
		Driver struct {
			Name            string `json:"name"`
			Version         string `json:"version"`
			SemanticVersion string `json:"semanticVersion"`
			Rules           []Rule `json:"rules"`
		} `json:"driver"`
	} `json:"tool"`
	Results     []RawResult  `json:"results"`
	Invocations []Invocation `json:"invocations"`
}

type Log struct {
	Version string `json:"version"`
	Runs    *[]Run `json:"runs"`
}

func Parse(input []byte) (*Log, error) {
	var log Log
	if err := json.Unmarshal(input, &log); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if log.Runs == nil || log.Version != "2.1.0" {
		return nil, ErrNotSARIF
	}
	return &log, nil
}

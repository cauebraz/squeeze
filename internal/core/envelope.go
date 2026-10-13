package core

import (
	"encoding/json"
	"math"
)

const SchemaVersion = 1

type envelope struct {
	SchemaVersion int      `json:"schema_version"`
	Format        string   `json:"format"`
	InputBytes    int      `json:"input_bytes"`
	TextBytes     int      `json:"text_bytes"`
	Ratio         float64  `json:"ratio"`
	Conditions    []string `json:"conditions"`
	FailedOn      []string `json:"failed_on"`
	Data          any      `json:"data"`
}

func Envelope(format string, inputBytes int, r Result, failOn []string) ([]byte, error) {
	conditions := r.Conditions
	if conditions == nil {
		conditions = []string{}
	}
	out, err := json.Marshal(envelope{
		SchemaVersion: SchemaVersion,
		Format:        format,
		InputBytes:    inputBytes,
		TextBytes:     len(r.Text),
		Ratio:         Ratio(len(r.Text), inputBytes),
		Conditions:    conditions,
		FailedOn:      Matched(conditions, failOn),
		Data:          r.Data,
	})
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

func Ratio(outputBytes, inputBytes int) float64 {
	if inputBytes <= 0 {
		return 0
	}
	return math.Round(float64(outputBytes)/float64(inputBytes)*10000) / 10000
}

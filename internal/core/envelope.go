package core

import (
	"encoding/json"
	"fmt"
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
	return append(escapeControls(out), '\n'), nil
}

func escapeControls(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for i := 0; i < len(b); i++ {
		switch {
		case b[i] == 0x7f:
			out = append(out, `\u007f`...)
		case b[i] == 0xc2 && i+1 < len(b) && b[i+1] >= 0x80 && b[i+1] < 0xa0:
			out = fmt.Appendf(out, `\u%04x`, b[i+1])
			i++
		default:
			out = append(out, b[i])
		}
	}
	return out
}

func Ratio(outputBytes, inputBytes int) float64 {
	if inputBytes <= 0 {
		return 0
	}
	return math.Round(float64(outputBytes)/float64(inputBytes)*10000) / 10000
}

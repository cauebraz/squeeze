package core

import (
	"bytes"
	"encoding/json"
)

func DecodeJSON(input []byte, v any) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(input))
	if err := dec.Decode(v); err != nil {
		return nil, err
	}
	return input[dec.InputOffset():], nil
}

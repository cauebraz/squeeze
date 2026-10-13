package jsonl

import "bytes"

func Lines(input []byte) [][]byte {
	var out [][]byte
	for _, raw := range bytes.Split(input, []byte("\n")) {
		if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 {
			out = append(out, trimmed)
		}
	}
	return out
}

func Detect(head []byte, n int, skip, match func([]byte) bool) bool {
	seen := 0
	for _, line := range bytes.Split(head, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 || (skip != nil && skip(line)) {
			continue
		}
		if match(line) {
			return true
		}
		seen++
		if seen == n {
			return false
		}
	}
	return false
}

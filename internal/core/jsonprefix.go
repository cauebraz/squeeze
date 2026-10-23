package core

func HasJSONPrefix(head, prefix []byte) bool {
	i := 0
	for _, c := range head {
		if i == len(prefix) {
			break
		}
		switch c {
		case ' ', '\t', '\r', '\n':
			continue
		}
		if c != prefix[i] {
			return false
		}
		i++
	}
	return i == len(prefix)
}

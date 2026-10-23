package core

import "strings"

func TrimRoot(s, root string) string {
	if root == "" {
		return s
	}
	s = strings.ReplaceAll(s, "file://"+root+"/", "")
	return strings.ReplaceAll(s, root+"/", "")
}

package tfplan

import (
	"strings"

	"github.com/cauebraz/squeeze/internal/sanitize"
)

var actionTable = map[string]action{
	"no-op":         actionNoop,
	"create":        actionCreate,
	"read":          actionRead,
	"update":        actionUpdate,
	"delete":        actionDelete,
	"delete,create": actionReplace,
	"create,delete": actionReplaceCBD,
	"forget":        actionForget,
	"forget,create": actionForgetThenCreate,
	"create,forget": actionCreateThenForget,
}

func mapAction(actions []string, importing bool) action {
	a, ok := actionTable[strings.Join(actions, ",")]
	switch {
	case !ok:
		return actionUnknown
	case a == actionNoop && importing:
		return actionImport
	}
	return a
}

func rawActions(actions []string, a action) []string {
	if a != actionUnknown {
		return nil
	}
	out := make([]string, 0, len(actions))
	for _, s := range actions {
		out = append(out, sanitize.Clean(s))
	}
	return out
}

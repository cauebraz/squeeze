package tfplan

import (
	"fmt"
	"strings"
)

var actionTable = map[string]Action{
	"no-op":         Noop,
	"create":        Create,
	"read":          Read,
	"update":        Update,
	"delete":        Delete,
	"delete,create": Replace,
	"create,delete": ReplaceCBD,
	"forget":        Forget,
}

func mapAction(actions []string, importing bool) (Action, error) {
	key := strings.Join(actions, ",")
	a, ok := actionTable[key]
	if !ok {
		return 0, fmt.Errorf("unsupported actions %q", actions)
	}
	if a == Noop && importing {
		return Import, nil
	}
	return a, nil
}

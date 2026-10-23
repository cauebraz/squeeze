package tfdiag

import "strings"

type ActionInvocation struct {
	Address  string `json:"address"`
	Event    string `json:"event,omitempty"`
	Resource string `json:"resource,omitempty"`
	Invoked  bool   `json:"invoked,omitempty"`
}

func (a ActionInvocation) Item(reason string) string {
	var notes []string
	switch {
	case a.Event != "" && a.Resource != "":
		notes = append(notes, a.Event+" "+a.Resource)
	case a.Event != "":
		notes = append(notes, a.Event)
	case a.Invoked:
		notes = append(notes, "invoked")
	}
	if reason != "" {
		notes = append(notes, reason)
	}
	if len(notes) == 0 {
		return a.Address
	}
	return a.Address + " (" + strings.Join(notes, "; ") + ")"
}

func WriteActions(b *strings.Builder, actions []ActionInvocation) {
	if len(actions) == 0 {
		return
	}
	items := make([]string, 0, len(actions))
	for _, a := range actions {
		items = append(items, a.Item(""))
	}
	b.WriteString("actions: ")
	b.WriteString(strings.Join(items, ", "))
	b.WriteByte('\n')
}

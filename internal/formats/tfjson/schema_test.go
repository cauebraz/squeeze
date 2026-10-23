package tfjson

import (
	"encoding/json"
	"testing"
)

func TestHookActionUnmarshal(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		input   string
		want    rawHookAction
		wantErr bool
	}{
		"name":        {`"read"`, rawHookAction{Name: "read"}, false},
		"address":     {`{"addr":"action.a.b"}`, rawHookAction{Addr: "action.a.b"}, false},
		"null":        {`null`, rawHookAction{}, false},
		"number":      {`1`, rawHookAction{}, false},
		"bad address": {`{"addr":1}`, rawHookAction{}, true},
	}
	for name, tt := range tests {
		var got rawHookAction
		err := json.Unmarshal([]byte(tt.input), &got)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("%s: Unmarshal() = %+v, %v", name, got, err)
		}
	}
}

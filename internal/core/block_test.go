package core

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestBlockWrite(t *testing.T) {
	t.Parallel()
	lines := []string{"a", "b", "c"}
	tests := map[string]struct {
		block Block
		tail  bool
		want  string
	}{
		"head cut":  {HeadBlock(lines, 2), false, "  a\n  b\n  ... 1 more line (--max-output 0)\n"},
		"tail cut":  {TailBlock(lines, 1), true, "  ... 2 earlier lines (--max-output 0)\n  c\n"},
		"no limit":  {HeadBlock(lines, 0), false, "  a\n  b\n  c\n"},
		"no lines":  {HeadBlock(nil, 5), false, ""},
		"tail fits": {TailBlock(lines, 3), true, "  a\n  b\n  c\n"},
	}
	for name, tt := range tests {
		var b strings.Builder
		if tt.tail {
			tt.block.WriteTail(&b, "  ", "--max-output")
		} else {
			tt.block.Write(&b, "  ", "--max-output")
		}
		if diff := cmp.Diff(tt.want, b.String()); diff != "" {
			t.Errorf("%s: (-want +got):\n%s", name, diff)
		}
	}
}

func TestBlockJSON(t *testing.T) {
	t.Parallel()
	for _, b := range []Block{HeadBlock(nil, 1), TailBlock([]string{"a", "b"}, 1)} {
		data, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if _, ok := got["lines"].([]any); !ok || len(got) != 2 || got["lines_omitted"] != float64(b.Omitted) {
			t.Errorf("JSON %s", data)
		}
	}
}

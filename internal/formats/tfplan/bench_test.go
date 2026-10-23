package tfplan

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cauebraz/squeeze/internal/core"
)

func largePlan(tb testing.TB, resources int) []byte {
	tb.Helper()
	changes := make([]map[string]any, 0, resources)
	for i := range resources {
		before := map[string]any{
			"id": fmt.Sprintf("i-%016d", i), "instance_type": "t3.small", "ami": "ami-0abcdef1234567890",
			"tags": map[string]any{"Name": fmt.Sprintf("web-%d", i), "Environment": "staging"},
			"ebs":  []any{map[string]any{"volume_size": 20, "volume_type": "gp3"}},
		}
		after := map[string]any{
			"id": fmt.Sprintf("i-%016d", i), "instance_type": "t3.medium", "ami": "ami-0abcdef1234567890",
			"tags": map[string]any{"Name": fmt.Sprintf("web-%d", i), "Environment": "production"},
			"ebs":  []any{map[string]any{"volume_size": 40, "volume_type": "gp3"}},
		}
		actions := []string{"no-op"}
		if i%4 == 0 {
			actions = []string{"update"}
		}
		changes = append(changes, map[string]any{
			"address": fmt.Sprintf("aws_instance.web[%d]", i),
			"change": map[string]any{
				"actions": actions, "before": before, "after": after,
				"after_unknown": map[string]any{}, "before_sensitive": map[string]any{}, "after_sensitive": map[string]any{},
			},
		})
	}
	data, err := json.Marshal(map[string]any{
		"format_version": "1.2", "terraform_version": "1.9.5",
		"planned_values": map[string]any{}, "resource_changes": changes,
	})
	if err != nil {
		tb.Fatal(err)
	}
	return data
}

func BenchmarkSqueeze2000(b *testing.B) {
	input := largePlan(b, 2000)
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	var textBytes int
	for b.Loop() {
		r, err := squeeze(input, defaultOptions())
		if err != nil {
			b.Fatal(err)
		}
		textBytes = len(r.Text)
	}
	b.ReportMetric(core.Ratio(textBytes, len(input)), "ratio")
}

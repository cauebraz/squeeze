package generic

import (
	"fmt"
	"strings"
	"testing"
)

func benchLog(n int) []byte {
	var b strings.Builder
	for i := range n {
		switch i % 10 {
		case 0:
			fmt.Fprintf(&b, "error: build step %d failed\n", i)
		case 1:
			fmt.Fprintf(&b, "warning: deprecated option in file%d.conf\n", i)
		case 2, 3, 4:
			b.WriteString("downloading module cache\n")
		default:
			fmt.Fprintf(&b, "compiling package %d of %d\n", i, n)
		}
	}
	return []byte(b.String())
}

func BenchmarkSqueeze(b *testing.B) {
	for _, n := range []int{1000, 10000} {
		input := benchLog(n)
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			for b.Loop() {
				if _, err := squeeze(input, defaultOptions()); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

package core

import "testing"

func TestLimitAndLimitTail(t *testing.T) {
	t.Parallel()
	items := []int{1, 2, 3, 4, 5}
	for _, tt := range []struct{ n, kept, omitted int }{{0, 5, 0}, {5, 5, 0}, {7, 5, 0}, {2, 2, 3}} {
		kept, omitted := Limit(items, tt.n)
		if len(kept) != tt.kept || omitted != tt.omitted {
			t.Errorf("Limit(%d) = %d, %d; want %d, %d", tt.n, len(kept), omitted, tt.kept, tt.omitted)
		}
		tail, omitted := LimitTail(items, tt.n)
		if len(tail) != tt.kept || omitted != tt.omitted || tail[len(tail)-1] != 5 {
			t.Errorf("LimitTail(%d) = %v, %d", tt.n, tail, omitted)
		}
	}
}

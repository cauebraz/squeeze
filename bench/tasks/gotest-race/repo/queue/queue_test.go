package queue

import (
	"fmt"
	"sync"
	"testing"
)

func TestPushPop(t *testing.T) {
	var q Queue
	q.Push(1)
	q.Push(2)
	if v, ok := q.Pop(); !ok || v != 1 {
		t.Fatalf("Pop() = %d, %v", v, ok)
	}
	if q.Len() != 1 {
		t.Fatalf("Len() = %d", q.Len())
	}
}

func TestEmptyPop(t *testing.T) {
	var q Queue
	if _, ok := q.Pop(); ok {
		t.Fatal("Pop() on empty returned ok")
	}
}

func TestConcurrentPush(t *testing.T) {
	var q Queue
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			q.Push(i)
		}(i)
	}
	wg.Wait()
	if q.Len() != 50 {
		t.Fatalf("Len() = %d, want 50", q.Len())
	}
}

func TestManyOrdered(t *testing.T) {
	for n := 0; n < 40; n++ {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			var q Queue
			for i := 0; i < n; i++ {
				q.Push(i)
			}
			for i := 0; i < n; i++ {
				if v, _ := q.Pop(); v != i {
					t.Fatalf("Pop() = %d, want %d", v, i)
				}
			}
		})
	}
}

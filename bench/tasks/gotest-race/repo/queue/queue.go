package queue

import "sync"

type Queue struct {
	mu    sync.Mutex
	items []int
}

func (q *Queue) Push(v int) {
	q.items = append(q.items, v)
}

func (q *Queue) Pop() (int, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return 0, false
	}
	v := q.items[0]
	q.items = q.items[1:]
	return v, true
}

func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

package core

func Limit[T any](items []T, limit int) ([]T, int) {
	if limit <= 0 || len(items) <= limit {
		return items, 0
	}
	return items[:limit], len(items) - limit
}

func LimitTail[T any](items []T, limit int) ([]T, int) {
	if limit <= 0 || len(items) <= limit {
		return items, 0
	}
	return items[len(items)-limit:], len(items) - limit
}

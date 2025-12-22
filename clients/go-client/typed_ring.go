package traceway

type TypedRing[T *E, E any] struct {
	arr      []T
	head     int
	capacity int
	len      int
}

func InitTypedRing[T *E, E any](capacity int) TypedRing[T, E] {
	return TypedRing[T, E]{
		arr:      make([]T, capacity),
		capacity: capacity,
	}
}
func (t *TypedRing[T, E]) Push(val T) {
	t.arr[t.head] = val
	t.head = (t.head + 1) % t.capacity
	if t.len < t.capacity {
		t.len += 1
	}
}
func (t *TypedRing[T, E]) ReadAll() []T {
	result := make([]T, t.len)
	for i := 0; i < t.len; i++ {
		idx := (t.head - t.len + i + t.capacity) % t.capacity
		result[i] = t.arr[idx]
	}
	return result
}
func (t *TypedRing[T, E]) Clear() {
	for i := range t.arr {
		t.arr[i] = nil
	}
	t.head = 0
	t.len = 0
}

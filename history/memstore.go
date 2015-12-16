package history

// MemStore provides a simple memory-based implementation
// of Store, suitable for testing.
type MemStore struct {
	// Events holds all the recorded events in order.
	Events []Event
}

// Append implements Store.Append.
func (s *MemStore) Append(e Event) {
	s.Events = append(s.Events, e)
}

// Append implements Store.ReverseIter.
func (s *MemStore) ReverseIter() Iterator {
	return &memIter{
		i:     len(s.Events),
		store: s,
	}
}

type memIter struct {
	i     int
	store *MemStore
}

func (iter *memIter) Close() error {
	return nil
}

func (iter *memIter) Next() bool {
	if iter.i <= 0 {
		return false
	}
	iter.i--
	return true
}

func (iter *memIter) Item() Event {
	return iter.store.Events[iter.i]
}

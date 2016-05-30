package history

// MemStore provides a simple memory-based implementation
// of Store, suitable for testing.
type MemStore struct {
	// Events holds all the recorded events in order.
	Events []Event

	toCommit []Event
}

// Append implements Store.Append.
func (s *MemStore) Append(e Event) {
	s.toCommit = append(s.toCommit, e)
}

// Commit commits all the events appended since the last
// call to Commit.
func (s *MemStore) Commit() error {
	s.Events = append(s.Events, s.toCommit...)
	s.toCommit = s.toCommit[:0]
	return nil
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

// Close implements Iterator.Close.
func (iter *memIter) Close() error {
	return nil
}

// Next implements Iterator.Next.
func (iter *memIter) Next() bool {
	if iter.i <= 0 {
		return false
	}
	iter.i--
	return true
}

// Item implements Iterator.Item.
func (iter *memIter) Item() Event {
	return iter.store.Events[iter.i]
}

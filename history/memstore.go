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
	return &eventsIter{
		i:      len(s.Events),
		events: s.Events,
	}
}

type eventsIter struct {
	events []Event
	i      int
}

// Close implements Iterator.Close.
func (iter *eventsIter) Close() error {
	return nil
}

// Next implements Iterator.Next.
func (iter *eventsIter) Next() bool {
	if iter.i <= 0 {
		return false
	}
	iter.i--
	return true
}

// Item implements Iterator.Item.
func (iter *eventsIter) Item() Event {
	return iter.events[iter.i]
}

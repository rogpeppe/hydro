// Package history provides an implementation of the ctl.History
// interface layered on top of a generic database interface.
package history

type Store interface {
	// Append atomically appends the given value to the database.
	Append(record []byte)

	// ReverseIter returns an iterator that enumerates all
	// items in the database from the end.
	ReverseIter() Iterator
}

type Iter struct {
	// Close closes the iterator, returning an error
	// if there was any error when iterating.
	// It is OK to call Close more than once.
	Close() error

	// Next moves to the next item in the iteration
	// and reports whether there is a next item.
	// It returns false if there was an error.
	Next() bool

	// Get gets the value of the current item.
	// If the iterator has finished, the result is undefined.
	Get() []byte
}

type event struct {
	Relay int
	Time int64
	On bool
}

// DB represents a store of historical events.
type DB struct {
	// relays holds one element for each relay, each containing an
	// ordered slice of events when the state changed.
	// Currently we hold the entire history in memory.
	relays [][]event
}

func New(store Store) (*DB, error) {
	var db DB
	// TODO limit iteration to recent past.
	for iter := store.ReverseIter(); iter.Next(); {
		var e event
		if err := e.UnmarshalText(iter.Item()); err != nil {
			return nil, fmt.Errorf("cannot unmarshal item %q: %v", iter.Item(), err)
		}
		if e.Relay >= len(db.relays) {
			relays := make([][]event, e.Relay + 1)
			copy(relays, db.relays)
			db.relays = relays
		}
		// TODO check that time is ordered?
		db.relays = append(db.relays, e)
	}
	return &db, nil
}

func (h *history) RecordState(relays ctl.RelayState, now time.Time) {
	for i := range h.relays {
		h.AddEvent(i, relays.IsSet(i), now)
	}
}

func (h *history) AddEvent(relay int, on bool, now time.Time) {
	lastOn, t := h.LatestChange(relay)
	if !now.After(t) {
		panic("cannot add out of order event")
	}
	if lastOn == on && !t.IsZero() {
		return
	}

	log.Printf("add event to %d %v %v", relay, on, D(now))
	e := event{
		Relay: i,
		Time: now,
		On: relays.IsSet(i),
	}
	data, err := e.MarshalBinary()
	if err != nil {
		panic(err)
	}
	h.store.Append(data)
	h.relays[relay] = append(h.relays[relay], event{
		on: on,
		t:  now,
	})
}

func (h *history) String() string {
	var buf bytes.Buffer
	for i, es := range h.relays {
		fmt.Fprintf(&buf, " [%d:", i)
		for _, e := range es {
			fmt.Fprintf(&buf, " ")
			if !e.on {
				fmt.Fprintf(&buf, "!")
			}
			fmt.Fprintf(&buf, "%v", D(e.t))
		}
		fmt.Fprintf(&buf, "]")
	}
	return strings.TrimPrefix(buf.String(), " ")
}

func (h *history) OnDuration(relay int, t0, t1 time.Time) time.Duration {
	d := h.onDuration(relay, t0, t1)
	log.Printf("history: %v", h)
	log.Printf("on duration(%d, %v, %v) -> %v (callers %s)", relay, D(t0), D(t1), d, debug.Callers(1, 2))
	return d
}

func (h *history) onDuration(relay int, t0, t1 time.Time) time.Duration {
	total := time.Duration(0)
	if relay >= len(h.relays) {
		return 0
	}
	times := h.relays[relay]
	// First find the first "off" event after t0.

	var onTime time.Time
	for _, e := range times {
		if e.on {
			// Be resilient to multiple on events in sequence.
			if onTime.IsZero() {
				onTime = e.t
			}
			continue
		}
		if onTime.IsZero() {
			continue
		}
		total += onDuration(onTime, e.t, t0, t1)
		onTime = time.Time{}
	}
	total += onDuration(onTime, t1, t0, t1)
	return total
}

func (h *history) LatestChange(relay int) (bool, time.Time) {
	if relay >= len(h.relays) {
		return false, time.Time{}
	}
	events := h.relays[relay]
	if len(events) == 0 {
		return false, time.Time{}
	}
	e := events[len(events)-1]
	return e.on, e.t
}

// onDuration returns the duration that [onTime, offTime] overlaps
// with [t0, t1]
func onDuration(onTime, offTime, t0, t1 time.Time) time.Duration {
	if onTime.IsZero() || !(onTime.Before(t1) && offTime.After(t0)) {
		return 0
	}
	if onTime.Before(t0) {
		onTime = t0
	}
	if offTime.After(t1) {
		offTime = t1
	}
	return offTime.Sub(onTime)
}

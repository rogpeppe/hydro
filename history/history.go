// Package history provides an implementation of the hydroctl.History
// interface layered on top of a generic database interface.
package history

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/rogpeppe/hydro/hydroctl"
)

// Store represents a persistent store of relay-changed events.
type Store interface {
	// Append adds the given event to the database.
	// Note that this will not actually write the event to
	// the database - the event should be committed
	// by the caller of history.RecordState.
	Append(Event)

	// ReverseIter returns an iterator that enumerates all
	// items in the database from the end. It does
	// not assume that events added by Append are immediately
	// visible.
	ReverseIter() Iterator
}

// Iterator represents an iterator moving back in time
// through relay events.
type Iterator interface {
	// Close closes the iterator, returning an error
	// if there was any error when iterating.
	// It is OK to call Close more than once.
	Close() error

	// Next moves to the next item in the iteration
	// and reports whether there is a next item.
	// It returns false if there was an error.
	Next() bool

	// Item gets the value of the current item.
	// If the iterator has finished, the result is undefined.
	Item() Event
}

// Event represents a relay-changed event.
type Event struct {
	// Relay holds the number of the relay that changed.
	Relay int
	// Time holds when it changed.
	Time time.Time
	// On holds whether the relay turned on or off
	// at that time.
	On bool
}

// DB represents a store of historical events.
// It notably implements hydroctl.History.
type DB struct {
	store Store

	// relays holds one element for each relay, each containing an
	// ordered slice of events when the state changed.
	// Currently we hold the entire history in memory.
	relays [][]Event
}

// New returns a new history database that uses the given
// store for persistent storage.
func New(store Store) (*DB, error) {
	db := &DB{
		store: store,
	}
	// TODO limit iteration to recent past.
	for iter := store.ReverseIter(); iter.Next(); {
		e := iter.Item()
		if e.Relay >= len(db.relays) {
			relays := make([][]Event, e.Relay+1)
			copy(relays, db.relays)
			db.relays = relays
		}
		// TODO check that time is ordered?
		db.relays[e.Relay] = append(db.relays[e.Relay], e)
	}
	// Reverse all events because we appended them in time-reversed
	// order.
	for _, events := range db.relays {
		reverse(events)
	}
	return db, nil
}

func reverse(a []Event) {
	for i, j := 0, len(a)-1; i < j; i, j = i+1, j-1 {
		a[i], a[j] = a[j], a[i]
	}
}

// RecordState records the given relay state in the history at
// the given time by appending events to the store. It does not
// commit the new events to the store.
func (h *DB) RecordState(relays hydroctl.RelayState, now time.Time) {
	for i := 0; i < hydroctl.MaxRelayCount; i++ {
		h.addEvent(i, relays.IsSet(i), now)
	}
}

func (h *DB) addEvent(relay int, on bool, now time.Time) {
	lastOn, t := h.LatestChange(relay)
	if !now.After(t) {
		panic("cannot add out of order event")
	}
	if lastOn == on && !(t.IsZero() && on) {
		// The state of the relay hasn't changed.
		// We don't bother recording the first
		// event if it's off.
		return
	}
	log.Printf("add event to %d %v %v", relay, on, timeFmt(now))

	if relay >= len(h.relays) {
		relays := make([][]Event, relay+1)
		copy(relays, h.relays)
		h.relays = relays
	}
	h.relays[relay] = append(h.relays[relay], Event{
		On:   on,
		Time: now,
	})
	h.store.Append(Event{
		Relay: relay,
		Time:  now,
		On:    on,
	})
}

// String returns a string representation of
func (h *DB) String() string {
	var buf bytes.Buffer
	for i, es := range h.relays {
		fmt.Fprintf(&buf, " [%d:", i)
		for _, e := range es {
			fmt.Fprintf(&buf, " ")
			if !e.On {
				fmt.Fprintf(&buf, "!")
			}
			fmt.Fprintf(&buf, "%v", timeFmt(e.Time))
		}
		fmt.Fprintf(&buf, "]")
	}
	return strings.TrimPrefix(buf.String(), " ")
}

// OnDuration implements hydroctl.History.OnDuration.
// It returns the length of time that the given
// relay has been switched on within the given time interval.
func (h *DB) OnDuration(relay int, t0, t1 time.Time) time.Duration {
	return h.onDuration(relay, t0, t1)
	//log.Printf("history: %v", h)
	//log.Printf("on duration(%d, %v, %v) -> %v (callers %s)", relay, D(t0), D(t1), d, debug.Callers(1, 2))
}

func (h *DB) onDuration(relay int, t0, t1 time.Time) time.Duration {
	total := time.Duration(0)
	if relay >= len(h.relays) {
		return 0
	}
	times := h.relays[relay]
	// First find the first "off" event after t0.

	var onTime time.Time
	for _, e := range times {
		if e.On {
			// Be resilient to multiple on events in sequence.
			if onTime.IsZero() {
				onTime = e.Time
			}
			continue
		}
		if onTime.IsZero() {
			continue
		}
		total += onDuration(onTime, e.Time, t0, t1)
		onTime = time.Time{}
	}
	total += onDuration(onTime, t1, t0, t1)
	return total
}

func (h *DB) LatestChange(relay int) (bool, time.Time) {
	if relay >= len(h.relays) {
		return false, time.Time{}
	}
	events := h.relays[relay]
	if len(events) == 0 {
		return false, time.Time{}
	}
	e := events[len(events)-1]
	return e.On, e.Time
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

func timeFmt(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}

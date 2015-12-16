// Package history provides an implementation of the ctl.History
// interface layered on top of a generic database interface.
package history

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"time"

	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/hydroctl"
)

type Store interface {
	// Append queues the given event to be added to the
	// database with Commit is called
	Append(Event)

	// Commit adds the events queued by Append since
	// the last commit to the database.
	Commit() error

	// ReverseIter returns an iterator that enumerates all
	// items in the database from the end.
	ReverseIter() Iterator
}

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

type Event struct {
	Relay int
	Time  time.Time
	On    bool
}

// DB represents a store of historical events.
type DB struct {
	store Store

	// relays holds one element for each relay, each containing an
	// ordered slice of events when the state changed.
	// Currently we hold the entire history in memory.
	relays [][]Event
}

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

func (h *DB) RecordState(relays hydroctl.RelayState, now time.Time) error {
	for i := 0; i < hydroctl.MaxRelayCount; i++ {
		h.addEvent(i, relays.IsSet(i), now)
	}
	// TODO perhaps this should not commit the state.
	// We could leave that up to the caller of RecordState.
	if err := h.store.Commit(); err != nil {
		return errgo.Notef(err, "cannot persistently record state")
	}
	return nil
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

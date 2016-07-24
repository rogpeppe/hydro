package history

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/rogpeppe/hydro/hydroctl"
)

// DiskStore provides a simple disk-based implementation
// of Store.
type DiskStore struct {
	path string
	f    *os.File
	// events holds all events in the store.
	events   []Event
	toCommit []Event
}

// NewDiskStore returns a disk-based store that stores
// events in the file with the given path and
// holds in memory all events after the given earliest
// time.
func NewDiskStore(path string, earliest time.Time) (*DiskStore, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_SYNC, 0666)
	if err != nil {
		return nil, fmt.Errorf("cannot open disk store: %v", err)
	}
	s := &DiskStore{
		f:    f,
		path: path,
	}
	older := make([]Event, hydroctl.MaxRelayCount)
	hasOlder := false
	scan := bufio.NewScanner(f)
	line := 1

	appendOlder := func() {
		if !hasOlder {
			return
		}
		hasOlder = false
		sort.Sort(eventsByTime(older))
		for i, e := range older {
			if !e.Time.IsZero() {
				log.Printf("appending out-of-date event %v", e)
				s.events = append(s.events, e)
			}
			older[i] = Event{}
		}
	}

	// Discard all events older than the specified earliest date
	// except we ensure that the last event prior
	// to that gets saved, otherwise we might have
	// a situation where a relay has been on for over
	// a week so we don't know its current state.
	for scan.Scan() {
		var e Event
		if err := e.UnmarshalText(scan.Bytes()); err != nil {
			log.Printf("%s:%d invalid event: %v", path, line, err)
			continue
		}
		if e.Time.Before(earliest) {
			older[e.Relay] = e
			log.Printf("retaining out-of-date event %#v", e)
			hasOlder = true
			continue
		}
		appendOlder()
		// Keep the last event prior if there was one.
		s.events = append(s.events, e)
	}
	appendOlder()
	return s, nil
}

type eventsByTime []Event

func (e eventsByTime) Len() int {
	return len(e)
}

func (e eventsByTime) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func (e eventsByTime) Less(i, j int) bool {
	return e[i].Time.Before(e[j].Time)
}

func (s *DiskStore) Close() error {
	return s.f.Close()
}

// Append implements Store.Append.
func (s *DiskStore) Append(e Event) {
	s.toCommit = append(s.toCommit, e)
}

// Commit commits all the events appended since the last
// call to Commit.
func (s *DiskStore) Commit() error {
	buf := make([]byte, 0, eventSize*len(s.toCommit))
	for _, e := range s.toCommit {
		buf = e.appendEvent(buf)
		buf = append(buf, '\n')
	}
	if n, err := s.f.Write(buf); err != nil {
		if n > 0 {
			log.Printf("warning: history file partially written (%d/%d bytes)", n, len(buf))
		}
		return fmt.Errorf("cannot write events: %v", err)
	}
	s.events = append(s.events, s.toCommit...)
	// TODO prune s.events.
	s.toCommit = s.toCommit[:0]
	return nil
}

func (s *DiskStore) ReverseIter() Iterator {
	return &eventsIter{
		i:      len(s.events),
		events: s.events,
	}
}

const eventSize = 2 + 1 + 1 + 1 + 20

func (e *Event) appendEvent(buf []byte) []byte {
	buf = strconv.AppendInt(buf, int64(e.Relay), 10)
	buf = append(buf, ' ')
	if e.On {
		buf = append(buf, '1')
	} else {
		buf = append(buf, '0')
	}
	buf = append(buf, ' ')
	buf = strconv.AppendInt(buf, e.Time.UnixNano()/1e6, 10)
	return buf
}

func (e *Event) UnmarshalText(buf []byte) error {
	var (
		relay int
		on    bool
		etime int64
	)
	if _, err := fmt.Sscanln(string(buf), &relay, &on, &etime); err != nil {
		return fmt.Errorf("cannot parse event %q", buf)
	}
	if relay >= hydroctl.MaxRelayCount || relay < 0 {
		return fmt.Errorf("invalid relay number %d in event", relay)
	}
	*e = Event{
		Relay: relay,
		On:    on,
		Time:  time.Unix(etime/1000, etime%1000*1e6),
	}
	return nil
}

package history_test

import (
	"io/ioutil"
	"path/filepath"
	"time"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/rogpeppe/hydro/history"
	"github.com/rogpeppe/hydro/hydroctl"
)

type suite struct{}

var _ = gc.Suite(suite{})

var _ hydroctl.History = (*history.DB)(nil)

type stateUpdate struct {
	t     time.Time
	state hydroctl.RelayState
}

type onDurationTest struct {
	relay          int
	t0             time.Time
	t1             time.Time
	expectDuration time.Duration
}

var historyTests = []struct {
	stateUpdates           []stateUpdate
	onDurationTests        []onDurationTest
	expectDBRelays         [][]history.Event
	expectLatestChangeOn   bool
	expectLatestChangeTime time.Time
}{{
	stateUpdates: []stateUpdate{{
		t:     T(2),
		state: mkRelays(0),
	}, {
		t:     T(5),
		state: 0,
	}, {
		t:     T(10),
		state: mkRelays(0),
	}},
	expectDBRelays: [][]history.Event{{{
		Time: T(2),
		On:   true,
	}, {
		Time: T(5),
		On:   false,
	}, {
		Time: T(10),
		On:   true,
	}}},
	expectLatestChangeOn:   true,
	expectLatestChangeTime: T(10),
	onDurationTests: []onDurationTest{{
		t0:             T(0),
		t1:             T(13),
		expectDuration: ((5 - 2) + (13 - 10)) * time.Hour,
	}, {
		t0:             T(3),
		t1:             T(4),
		expectDuration: (4 - 3) * time.Hour,
	}, {
		t0:             T(5),
		t1:             T(10),
		expectDuration: 0,
	}, {
		t0:             T(7),
		t1:             T(11),
		expectDuration: 1 * time.Hour,
	}},
}}

var epoch = time.Date(2000, 01, 01, 0, 0, 0, 0, time.UTC)

func T(i int) time.Time {
	return epoch.Add(time.Duration(i) * time.Hour)
}

func D(t time.Time) time.Duration {
	return t.Sub(epoch)
}

func (suite) TestHistory(c *gc.C) {
	for i, test := range historyTests {
		c.Logf("test %d", i)
		var store history.MemStore
		h, err := history.New(&store)
		c.Assert(err, gc.IsNil)
		for _, update := range test.stateUpdates {
			h.RecordState(update.state, update.t)
			store.Commit()
		}
		// Check that we've ended up with the right history.
		c.Assert(history.DBRelays(h), jc.DeepEquals, test.expectDBRelays)
		// Check that we get the same thing when reading from
		// the store.
		h, err = history.New(&store)
		c.Assert(err, gc.IsNil)
		c.Assert(history.DBRelays(h), jc.DeepEquals, test.expectDBRelays)
		on, t := h.LatestChange(0)
		c.Assert(on, gc.Equals, test.expectLatestChangeOn)
		c.Assert(t.Equal(test.expectLatestChangeTime), gc.Equals, true)
		for i, dtest := range test.onDurationTests {
			c.Logf("dtest %d", i)
			c.Check(h.OnDuration(dtest.relay, dtest.t0, dtest.t1), gc.Equals, dtest.expectDuration)
		}
		c.Logf("")
	}
}

func (suite) TestDiskStoreCreate(c *gc.C) {
	d := c.MkDir()
	path := filepath.Join(d, "history")
	store, err := history.NewDiskStore(path, time.Now())
	c.Assert(err, gc.IsNil)

	t0 := time.Unix(1000, int64(time.Millisecond))

	events := []history.Event{{
		Relay: 2,
		On:    true,
		Time:  t0,
	}, {
		Relay: 3,
		On:    true,
		Time:  t0.Add(time.Second),
	}, {
		Relay: 3,
		On:    false,
		Time:  t0.Add(2 * time.Second),
	}}

	store.Append(events[0])
	store.Append(events[1])
	err = store.Commit()
	c.Assert(err, gc.IsNil)

	store.Append(events[2])
	err = store.Commit()
	c.Assert(err, gc.IsNil)

	err = store.Close()
	c.Assert(err, gc.IsNil)

	data, err := ioutil.ReadFile(path)
	c.Assert(err, gc.IsNil)
	c.Assert(string(data), gc.Equals, `
2 1 1000001
3 1 1001001
3 0 1002001
`[1:])

	// Reading when the earliest time is before the earliest
	// event should give us all events.
	store, err = history.NewDiskStore(path, t0.Add(-5*time.Second))
	c.Assert(err, gc.IsNil)

	c.Assert(allEvents(store), jc.DeepEquals, events)

	store.Close()

	// Reading when the earliest time is after the earliest
	// event should give us only the latest event earlier than "earliest".

	store, err = history.NewDiskStore(path, t0.Add(5*time.Second))
	c.Assert(err, gc.IsNil)

	c.Assert(allEvents(store), jc.DeepEquals, []history.Event{{
		Relay: 2,
		On:    true,
		Time:  t0,
	}, {
		Relay: 3,
		On:    false,
		Time:  t0.Add(2 * time.Second),
	}})

	store.Close()

	// Add some more events.
	store, err = history.NewDiskStore(path, t0.Add(5*time.Second))
	c.Assert(err, gc.IsNil)

	store.Append(history.Event{
		Relay: 4,
		On:    true,
		Time:  t0.Add(6 * time.Second),
	})
	err = store.Commit()
	c.Assert(err, gc.IsNil)

	store.Close()

	data, err = ioutil.ReadFile(path)
	c.Assert(err, gc.IsNil)

	store, err = history.NewDiskStore(path, t0.Add(5*time.Second))
	c.Assert(err, gc.IsNil)

	c.Logf("store: %#v", store)

	c.Assert(allEvents(store), jc.DeepEquals, []history.Event{{
		Relay: 2,
		On:    true,
		Time:  t0,
	}, {
		Relay: 3,
		On:    false,
		Time:  t0.Add(2 * time.Second),
	}, {
		Relay: 4,
		On:    true,
		Time:  t0.Add(6 * time.Second),
	}})
}

func allEvents(store history.Store) []history.Event {
	iter := store.ReverseIter()
	defer iter.Close()
	var gotEvents []history.Event
	for iter.Next() {
		gotEvents = append([]history.Event{iter.Item()}, gotEvents...)
	}
	return gotEvents
}

func mkRelays(relays ...uint) hydroctl.RelayState {
	var state hydroctl.RelayState
	for _, r := range relays {
		state |= 1 << r
	}
	return state
}

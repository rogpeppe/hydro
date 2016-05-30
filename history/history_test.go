package history_test

import (
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

func mkRelays(relays ...uint) hydroctl.RelayState {
	var state hydroctl.RelayState
	for _, r := range relays {
		state |= 1 << r
	}
	return state
}

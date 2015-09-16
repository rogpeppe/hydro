package ctl_test

import (
	"bytes"
	"fmt"
	"local/runtime/debug"
	"log"
	"strings"
	"time"

	"github.com/rogpeppe/hydro/ctl"
	gc "gopkg.in/check.v1"
)

type suite struct{}

var _ = gc.Suite(suite{})

type onDurationTest struct {
	relay          int
	t0             time.Time
	t1             time.Time
	expectDuration time.Duration
}

var epoch = time.Date(2000, 01, 01, 0, 0, 0, 0, time.UTC)

func T(i int) time.Time {
	return epoch.Add(time.Duration(i) * time.Hour)
}

func D(t time.Time) time.Duration {
	return t.Sub(epoch)
}

var historyTests = []struct {
	history                history
	onDurationTests        []onDurationTest
	expectLatestChangeOn   bool
	expectLatestChangeTime time.Time
}{{
	history: history{
		relays: [][]event{{{
			t:  T(2),
			on: true,
		}, {
			t:  T(5),
			on: false,
		}, {
			t:  T(10),
			on: true,
		}}},
	},
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

func (suite) TestHistory(c *gc.C) {
	for i, test := range historyTests {
		c.Logf("test %d", i)
		on, t := test.history.LatestChange(0)
		c.Assert(on, gc.Equals, test.expectLatestChangeOn)
		c.Assert(t.Equal(test.expectLatestChangeTime), gc.Equals, true)
		for i, dtest := range test.onDurationTests {
			c.Logf("dtest %d", i)
			c.Check(test.history.OnDuration(dtest.relay, dtest.t0, dtest.t1), gc.Equals, dtest.expectDuration)
		}
		c.Logf("")
	}
}

type assessNowTest struct {
	now time.Time
	// When transition is true, the relays will be
	// assessed just before and at the now
	// time. Beforehand, the expected state is that
	// of the previous assessNowTest entry; at
	// the now time, the state should change to expectState.
	transition  bool
	meters      ctl.MeterReading
	expectState ctl.RelayState
}

var assessTests = []struct {
	about          string
	history        history
	currentState   ctl.RelayState
	cfg            ctl.Config
	assessNowTests []assessNowTest
}{{
	about: "everything off, some relays that are always on",
	cfg: ctl.Config{
		Relays: []ctl.RelayConfig{
			0: {
				Mode:     ctl.AlwaysOn,
				MaxPower: 100,
			},
			5: {
				Mode:     ctl.AlwaysOn,
				MaxPower: 100,
			}},
	},
	currentState: mkRelays(),
	assessNowTests: []assessNowTest{{
		// The first relay comes on immediately.
		now:         T(0),
		expectState: mkRelays(0),
	}, {
		now:         T(0).Add(ctl.MinimumChangeDuration),
		expectState: mkRelays(0, 5),
		transition:  true,
	}, {
		now:         T(1),
		expectState: mkRelays(0, 5),
	}},
}, {
	about: "everything on, one relay that's always off",
	cfg: ctl.Config{
		Relays: []ctl.RelayConfig{{
			Mode:     ctl.AlwaysOff,
			MaxPower: 100,
		}},
	},
	currentState: mkRelays(0),
	assessNowTests: []assessNowTest{{
		now:         T(0),
		expectState: mkRelays(),
	}, {
		now:         T(1000),
		expectState: mkRelays(),
	}},
}, {
	about: "relay on for exactly 2 hours between 1am and 5am",
	cfg: ctl.Config{
		Relays: []ctl.RelayConfig{{
			Mode:     ctl.InUse,
			MaxPower: 100,
			InUse: []*ctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         ctl.Exactly,
				Duration:     2 * time.Hour,
			}},
		}},
	},
	assessNowTests: []assessNowTest{{
		now: T(0),
	}, {
		// It should switch on at 1am exactly.
		now:         T(1),
		transition:  true,
		expectState: mkRelays(0),
	}, {
		// ... and remain on for the duration.
		now:         T(2),
		expectState: mkRelays(0),
	}, {
		// and switches off at 3am.
		now:        T(3),
		transition: true,
	}, {
		// ... remaining off until 1am the next day.
		now: T(5),
	}, {
		now: T(6),
	}, {
		now: T(10),
	}, {
		now: T(24),
	}, {
		// switch on at 1am the next day.
		now:         T(25),
		expectState: mkRelays(0),
		transition:  true,
	}, {
		// and off at 3am the next day.
		now:        T(27),
		transition: true,
	}},
}, {
	about: "relay on for at least 2 hours between 1am and 5am",
	cfg: ctl.Config{
		Relays: []ctl.RelayConfig{{
			Mode:     ctl.InUse,
			MaxPower: 100,
			InUse: []*ctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         ctl.AtLeast,
				Duration:     2 * time.Hour,
			}},
		}},
	},
	assessNowTests: []assessNowTest{{
		now: T(0),
	}, {
		// It should switch on at 1am exactly.
		now:         T(1),
		expectState: mkRelays(0),
		transition:  true,
	}, {
		// ... and remain on for the duration.
		now:         T(2),
		expectState: mkRelays(0),
	}, {
		now:         T(3),
		expectState: mkRelays(0),
	}, {
		// and switch off at 5am, remaining off until 1am the next day.
		now:        T(5),
		transition: true,
	}, {
		now: T(6),
	}, {
		now: T(10),
	}, {
		now: T(24),
	}, {
		// switch on at 1am the next day.
		now:         T(25),
		expectState: mkRelays(0),
		transition:  true,
	}, {
		// and off at 5am the next day.
		now:        T(29),
		transition: true,
	}},
}, {
	about: "relay on for at most 2 hours between 1am and 5am",
	cfg: ctl.Config{
		Relays: []ctl.RelayConfig{{
			Mode:     ctl.InUse,
			MaxPower: 100,
			InUse: []*ctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         ctl.AtMost,
				Duration:     2 * time.Hour,
			}},
		}},
	},
	assessNowTests: []assessNowTest{{
		now: T(0),
	}, {
		// It should switch on at 1am exactly.
		now:         T(1),
		expectState: mkRelays(0),
		transition:  true,
	}, {
		// ... and remain on for the duration.
		now:         T(2),
		expectState: mkRelays(0),
	}, {
		// and switches off at 3am.
		now:        T(3),
		transition: true,
	}, {
		// ... remaining off until 1am the next day.
		now: T(5),
	}, {
		now: T(6),
	}, {
		now: T(10),
	}, {
		now: T(24),
	}, {
		// switch on at 1am the next day.
		now:         T(25),
		expectState: mkRelays(0),
		transition:  true,
	}, {
		// and off at 3am the next day.
		now:        T(27),
		transition: true,
	}},
}, {
	about: "when lots of power is in use, discretionary power doesn't kick in until it must",
	cfg: ctl.Config{
		Relays: []ctl.RelayConfig{{
			// Relay 0 is on for at least 2 hours between 1am and 5am.
			Mode:     ctl.InUse,
			MaxPower: 100,
			InUse: []*ctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         ctl.AtLeast,
				Duration:     2 * time.Hour,
			}},
		}, {
			Mode:     ctl.AlwaysOn,
			MaxPower: 1000,
		}},
	},
	assessNowTests: []assessNowTest{{
		// At midnight, just the always-on relay is on.
		now: T(0),
		meters: ctl.MeterReading{
			Import: 1000,
			Here:   1000,
		},
		expectState: mkRelays(1),
	}, {
		// At 1am, the discretionary power relay
		// doesn't kick in because we don't want
		// to import anything.
		now: T(1),
		meters: ctl.MeterReading{
			Import: 1000,
			Here:   1000,
		},
		expectState: mkRelays(1),
	}, {
		// Just before 3am, power is still off.
		now: T(3).Add(-1),
		meters: ctl.MeterReading{
			Import: 1000,
			Here:   1000,
		},
		expectState: mkRelays(1),
	}, {
		// At 3am, the discretionary power relay
		// kicks in because we realise that it needs
		// to be on or it will miss its slot criteria.
		now: T(3),
		meters: ctl.MeterReading{
			Import: 1100,
			Here:   1100,
		},
		expectState: mkRelays(0, 1),
	}, {
		// It's still on just before 5am (the end of the slot).
		now: T(5).Add(-1),
		meters: ctl.MeterReading{
			Import: 1100,
			Here:   1100,
		},
		expectState: mkRelays(0, 1),
	}, {
		// ... and switches off at the end of the slot.
		now: T(5),
		meters: ctl.MeterReading{
			Import: 1100,
			Here:   1100,
		},
		expectState: mkRelays(1),
	}},
}, {
	about: "When several relays are discretionary, they turn on one at a time",
	cfg: ctl.Config{
		Relays: []ctl.RelayConfig{{
			// Relay 0 is on for at least 2 hours between 1am and 5am.
			Mode:     ctl.InUse,
			MaxPower: 100,
			InUse: []*ctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         ctl.Exactly,
				Duration:     2 * time.Hour,
			}},
		}, {
			// Relay 1 is the same as relay 0.
			Mode:     ctl.InUse,
			MaxPower: 100,
			InUse: []*ctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         ctl.Exactly,
				Duration:     2 * time.Hour,
			}},
		}},
	},
	assessNowTests: []assessNowTest{{
		// At 1am, relay 0 (arbitrary choice) turns on.
		now:         T(1),
		expectState: mkRelays(0),
		transition:  true,
	}, {
		// After MinimumChangeDuration, the other relay
		// turns on.
		now:         T(1).Add(ctl.MinimumChangeDuration),
		expectState: mkRelays(0, 1),
		transition:  true,
	}, {
		// After the first relay has been on for its allotted two hours,
		// it turns off.
		now:         T(3),
		expectState: mkRelays(1),
		transition:  true,
	}, {
		// Same for the second relay.
		now:         T(3).Add(ctl.MinimumChangeDuration),
		expectState: mkRelays(),
		transition:  true,
	}},
}, {
	about: "When a discretionary-power relay is on and there's not enough power, it switches off until there is",
	cfg: ctl.Config{
		Relays: []ctl.RelayConfig{{
			// Relay 0 is on for at least 2 hours between 1am and 5am.
			Mode:     ctl.InUse,
			MaxPower: 100,
			InUse: []*ctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         ctl.Exactly,
				Duration:     2 * time.Hour,
			}},
		},
		}},
	assessNowTests: []assessNowTest{{
		// The relay comes on as usual at 1am.
		now:         T(1),
		expectState: mkRelays(0),
		transition:  true,
	}, {
		// At 2am, the meter shows that we're importing
		// electricity, so the relay is switched off.
		now: T(2),
		meters: ctl.MeterReading{
			Import: 500,
			Here:   1000,
		},
		expectState: mkRelays(),
		transition:  true,
	}, {
		// At 3am, the generator starts generating
		// enough electricity again,
		// so we switch the relay back on.
		now: T(3),
		meters: ctl.MeterReading{
			Here: 1000,
		},
		expectState: mkRelays(0),
		transition:  true,
	}, {
		// At 4am we've satisfied the slot requirements,
		// so we turn it off again.
		now: T(4),
		meters: ctl.MeterReading{
			Here: 1000,
		},
		expectState: mkRelays(),
		transition:  true,
	}},
}, {
	about: "When several discretionary-power relays are on and power is limited, we switch enough off to try to regain the power",
	cfg: ctl.Config{
		Relays: []ctl.RelayConfig{{
			Mode:     ctl.InUse,
			MaxPower: 1000,
			InUse: []*ctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         ctl.Exactly,
				Duration:     2 * time.Hour,
			}},
		}, {
			Mode:     ctl.InUse,
			MaxPower: 1000,
			InUse: []*ctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         ctl.Exactly,
				Duration:     2 * time.Hour,
			}},
		}, {
			Mode:     ctl.InUse,
			MaxPower: 1000,
			InUse: []*ctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         ctl.Exactly,
				Duration:     2 * time.Hour,
			}},
		}},
	},
	assessNowTests: []assessNowTest{{
		// At the start of the slot, each relay
		// will come on in turn.
		now: T(1),
		meters: ctl.MeterReading{
			// The generator is producing 3kW.
			Import: -3000,
		},
		expectState: mkRelays(0),
		transition:  true,
	}, {
		now: T(1).Add(ctl.MinimumChangeDuration),
		meters: ctl.MeterReading{
			Import: -2000,
			Here:   1000,
		},
		expectState: mkRelays(0, 1),
		transition:  true,
	}, {
		now: T(1).Add(2 * ctl.MinimumChangeDuration),
		meters: ctl.MeterReading{
			Import: -1000,
			Here:   2000,
		},
		expectState: mkRelays(0, 1, 2),
		transition:  true,
	}, {
		// A little while after, we're using all the generated
		// power but there's no problem with that.
		now: T(1).Add(2*ctl.MinimumChangeDuration + time.Minute),
		meters: ctl.MeterReading{
			Import: 0,
			Here:   3000,
		},
		expectState: mkRelays(0, 1, 2),
		transition:  true,
	}, {
		// At 2am, the other house starts using power,
		// so we switch off just enough relays to
		// hope that we stop using the excess power.
		now: T(2),
		meters: ctl.MeterReading{
			Import:    1500,
			Here:      3000,
			Neighbour: 1500,
		},
		expectState: mkRelays(2),
		transition:  true,
	}},
}}

//, {
//	about: "If we've switched on a relay recently, we can't switch if off immediately",
//	cfg: ctl.Config{
//		Relays: []ctl.RelayConfig{{
//			Mode:     ctl.InUse,
//			MaxPower: 1000,
//			InUse: []*ctl.Slot{{
//				Start:        1 * time.Hour,
//				SlotDuration: 4 * time.Hour,
//				Kind:         ctl.Exactly,
//				Duration:     2 * time.Hour,
//			}},
//		},
//	},
//	assessNowTests: []assessNowTest{{
//		now:         T(1),
//		expectState: mkRelays(0),
//		transition: true,
//	}, {
//		// The next meter reading would normally make
//		//
//}}

func (suite) TestAssess(c *gc.C) {
	for i, test := range assessTests {
		c.Logf("")
		c.Logf("test %d: %s", i, test.about)
		state := test.currentState
		history := test.history
		if history.relays == nil {
			history = *newHistory(len(test.cfg.Relays))
		}
		c.Assert(history.relays, gc.HasLen, len(test.cfg.Relays))
		for j, innertest := range test.assessNowTests {
			c.Logf("\t%d. at %v", j, D(innertest.now))
			if innertest.transition {
				var prevMeters ctl.MeterReading
				if j > 0 {
					prevMeters = test.assessNowTests[j-1].meters
				}
				// Check just before the test time to make
				// sure the state is unchanged from the
				// previous test.
				newState := ctl.Assess(&test.cfg, state, &history, prevMeters, innertest.now.Add(-1))
				c.Assert(newState, gc.Equals, state, gc.Commentf("previous state"))
			}
			state = ctl.Assess(&test.cfg, state, &history, innertest.meters, innertest.now)
			c.Assert(state, gc.Equals, innertest.expectState)
			history.RecordState(state, innertest.now)
			c.Logf("new history: %v", &history)
		}
	}
}

type event struct {
	t  time.Time
	on bool
}

type history struct {
	// relays holds one element for each relay, each containing an
	// ordered slice of events when the state changed.
	relays [][]event
}

func mkRelays(relays ...uint) ctl.RelayState {
	var state ctl.RelayState
	for _, r := range relays {
		state |= 1 << r
	}
	return state
}

func newHistory(nrelays int) *history {
	h := &history{
		relays: make([][]event, nrelays),
	}
	return h
}

func (h *history) RecordState(relays ctl.RelayState, now time.Time) {
	for i := range h.relays {
		h.AddEvent(i, relays.IsSet(i), now)
	}
}

func (h *history) AddEvent(relay int, on bool, now time.Time) {
	log.Printf("add event to %d %v %v", relay, on, D(now))
	lastOn, t := h.LatestChange(relay)
	if !now.After(t) {
		panic("cannot add out of order event")
	}
	if lastOn == on && !t.IsZero() {
		return
	}
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

func mustParseTime(layout, s string) time.Time {
	t, err := time.Parse(layout, s)
	if err != nil {
		panic(err)
	}
	return t
}

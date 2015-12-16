package hydroctl_test

import (
	"time"

	gc "gopkg.in/check.v1"

	"github.com/rogpeppe/hydro/history"
	"github.com/rogpeppe/hydro/hydroctl"
)

type suite struct{}

var _ = gc.Suite(suite{})

var epoch = time.Date(2000, 01, 01, 0, 0, 0, 0, time.UTC)

func T(i int) time.Time {
	return epoch.Add(time.Duration(i) * time.Hour)
}

func D(t time.Time) time.Duration {
	return t.Sub(epoch)
}

type assessNowTest struct {
	now time.Time
	// When transition is true, the relays will be
	// assessed just before and at the now
	// time. Beforehand, the expected state is that
	// of the previous assessNowTest entry; at
	// the now time, the state should change to expectState.
	transition  bool
	meters      hydroctl.MeterReading
	expectState hydroctl.RelayState
}

type stateUpdate struct {
	t     time.Time
	state hydroctl.RelayState
}

var assessTests = []struct {
	about           string
	previousUpdates []stateUpdate
	currentState    hydroctl.RelayState
	cfg             hydroctl.Config
	assessNowTests  []assessNowTest
}{{
	about: "everything off, some relays that are always on",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{
			0: {
				Mode:     hydroctl.AlwaysOn,
				MaxPower: 100,
			},
			5: {
				Mode:     hydroctl.AlwaysOn,
				MaxPower: 100,
			}},
	},
	currentState: mkRelays(),
	assessNowTests: []assessNowTest{{
		// The first relay comes on immediately.
		now:         T(0),
		expectState: mkRelays(0),
	}, {
		now:         T(0).Add(hydroctl.MinimumChangeDuration),
		expectState: mkRelays(0, 5),
		transition:  true,
	}, {
		now:         T(1),
		expectState: mkRelays(0, 5),
	}},
}, {
	about: "everything on, one relay that's always off",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode:     hydroctl.AlwaysOff,
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
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode:     hydroctl.InUse,
			MaxPower: 100,
			InUse: []*hydroctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         hydroctl.Exactly,
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
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode:     hydroctl.InUse,
			MaxPower: 100,
			InUse: []*hydroctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         hydroctl.AtLeast,
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
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode:     hydroctl.InUse,
			MaxPower: 100,
			InUse: []*hydroctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         hydroctl.AtMost,
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
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			// Relay 0 is on for at least 2 hours between 1am and 5am.
			Mode:     hydroctl.InUse,
			MaxPower: 100,
			InUse: []*hydroctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         hydroctl.AtLeast,
				Duration:     2 * time.Hour,
			}},
		}, {
			Mode:     hydroctl.AlwaysOn,
			MaxPower: 1000,
		}},
	},
	assessNowTests: []assessNowTest{{
		// At midnight, just the always-on relay is on.
		now: T(0),
		meters: hydroctl.MeterReading{
			Import: 1000,
			Here:   1000,
		},
		expectState: mkRelays(1),
	}, {
		// At 1am, the discretionary power relay
		// doesn't kick in because we don't want
		// to import anything.
		now: T(1),
		meters: hydroctl.MeterReading{
			Import: 1000,
			Here:   1000,
		},
		expectState: mkRelays(1),
	}, {
		// Just before 3am, power is still off.
		now: T(3).Add(-1),
		meters: hydroctl.MeterReading{
			Import: 1000,
			Here:   1000,
		},
		expectState: mkRelays(1),
	}, {
		// At 3am, the discretionary power relay
		// kicks in because we realise that it needs
		// to be on or it will miss its slot criteria.
		now: T(3),
		meters: hydroctl.MeterReading{
			Import: 1100,
			Here:   1100,
		},
		expectState: mkRelays(0, 1),
	}, {
		// It's still on just before 5am (the end of the slot).
		now: T(5).Add(-1),
		meters: hydroctl.MeterReading{
			Import: 1100,
			Here:   1100,
		},
		expectState: mkRelays(0, 1),
	}, {
		// ... and switches off at the end of the slot.
		now: T(5),
		meters: hydroctl.MeterReading{
			Import: 1100,
			Here:   1100,
		},
		expectState: mkRelays(1),
	}},
}, {
	about: "When several relays are discretionary, they turn on one at a time",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			// Relay 0 is on for at least 2 hours between 1am and 5am.
			Mode:     hydroctl.InUse,
			MaxPower: 100,
			InUse: []*hydroctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         hydroctl.Exactly,
				Duration:     2 * time.Hour,
			}},
		}, {
			// Relay 1 is the same as relay 0.
			Mode:     hydroctl.InUse,
			MaxPower: 100,
			InUse: []*hydroctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         hydroctl.Exactly,
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
		now:         T(1).Add(hydroctl.MinimumChangeDuration),
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
		now:         T(3).Add(hydroctl.MinimumChangeDuration),
		expectState: mkRelays(),
		transition:  true,
	}},
}, {
	about: "When a discretionary-power relay is on and there's not enough power, it switches off until there is",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			// Relay 0 is on for at least 2 hours between 1am and 5am.
			Mode:     hydroctl.InUse,
			MaxPower: 100,
			InUse: []*hydroctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         hydroctl.Exactly,
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
		meters: hydroctl.MeterReading{
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
		meters: hydroctl.MeterReading{
			Here: 1000,
		},
		expectState: mkRelays(0),
		transition:  true,
	}, {
		// At 4am we've satisfied the slot requirements,
		// so we turn it off again.
		now: T(4),
		meters: hydroctl.MeterReading{
			Here: 1000,
		},
		expectState: mkRelays(),
		transition:  true,
	}},
}, {
	about: "When several discretionary-power relays are on and power is limited, we switch enough off to try to regain the power",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode:     hydroctl.InUse,
			MaxPower: 1000,
			InUse: []*hydroctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         hydroctl.Exactly,
				Duration:     2 * time.Hour,
			}},
		}, {
			Mode:     hydroctl.InUse,
			MaxPower: 1000,
			InUse: []*hydroctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         hydroctl.Exactly,
				Duration:     2 * time.Hour,
			}},
		}, {
			Mode:     hydroctl.InUse,
			MaxPower: 1000,
			InUse: []*hydroctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         hydroctl.Exactly,
				Duration:     2 * time.Hour,
			}},
		}},
	},
	assessNowTests: []assessNowTest{{
		// At the start of the slot, each relay
		// will come on in turn.
		now: T(1),
		meters: hydroctl.MeterReading{
			// The generator is producing 3kW.
			Import: -3000,
		},
		expectState: mkRelays(0),
		transition:  true,
	}, {
		now: T(1).Add(hydroctl.MinimumChangeDuration),
		meters: hydroctl.MeterReading{
			Import: -2000,
			Here:   1000,
		},
		expectState: mkRelays(0, 1),
		transition:  true,
	}, {
		now: T(1).Add(2 * hydroctl.MinimumChangeDuration),
		meters: hydroctl.MeterReading{
			Import: -1000,
			Here:   2000,
		},
		expectState: mkRelays(0, 1, 2),
		transition:  true,
	}, {
		// A little while after, we're using all the generated
		// power but there's no problem with that.
		now: T(1).Add(2*hydroctl.MinimumChangeDuration + time.Minute),
		meters: hydroctl.MeterReading{
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
		meters: hydroctl.MeterReading{
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
//	cfg: hydroctl.Config{
//		Relays: []hydroctl.RelayConfig{{
//			Mode:     hydroctl.InUse,
//			MaxPower: 1000,
//			InUse: []*hydroctl.Slot{{
//				Start:        1 * time.Hour,
//				SlotDuration: 4 * time.Hour,
//				Kind:         hydroctl.Exactly,
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

		history, err := history.New(&history.MemStore{})
		c.Assert(err, gc.IsNil)
		for _, u := range test.previousUpdates {
			history.RecordState(u.state, u.t)
		}
		for j, innertest := range test.assessNowTests {
			c.Logf("\t%d. at %v", j, D(innertest.now))
			if innertest.transition {
				var prevMeters hydroctl.MeterReading
				if j > 0 {
					prevMeters = test.assessNowTests[j-1].meters
				}
				// Check just before the test time to make
				// sure the state is unchanged from the
				// previous test.
				newState := hydroctl.Assess(&test.cfg, state, history, prevMeters, innertest.now.Add(-1))
				c.Assert(newState, gc.Equals, state, gc.Commentf("previous state"))
			}
			state = hydroctl.Assess(&test.cfg, state, history, innertest.meters, innertest.now)
			c.Assert(state, gc.Equals, innertest.expectState)
			history.RecordState(state, innertest.now)
			c.Logf("new history: %v", &history)
		}
	}
}

func mkRelays(relays ...uint) hydroctl.RelayState {
	var state hydroctl.RelayState
	for _, r := range relays {
		state |= 1 << r
	}
	return state
}

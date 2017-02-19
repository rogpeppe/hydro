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
	powerUse    hydroctl.PowerUseSample
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
			Mode: hydroctl.InUse,
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
	about: "relay on through midnight",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode: hydroctl.InUse,
			InUse: []*hydroctl.Slot{{
				Start:        22 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         hydroctl.Exactly,
				Duration:     4 * time.Hour,
			}},
		}},
	},
	assessNowTests: []assessNowTest{{
		now: T(21),
	}, {
		// It should switch on at 1am exactly.
		now:         T(22),
		transition:  true,
		expectState: mkRelays(0),
	}, {
		// ... and remain on for the duration.
		now:         T(23),
		expectState: mkRelays(0),
	}, {
		// ... and remain on for the duration.
		now:         T(25),
		expectState: mkRelays(0),
	}, {
		// and switches off at 2am.
		now:        T(26),
		transition: true,
	}},
}, {
	about: "relay on for at least 2 hours between 1am and 5am",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode: hydroctl.InUse,
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
			Mode: hydroctl.InUse,
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
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Here: 1000,
			},
		},
		expectState: mkRelays(1),
	}, {
		// At 1am, the discretionary power relay
		// doesn't kick in because we don't want
		// to import anything.
		now: T(1),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Here: 1000,
			},
		},
		expectState: mkRelays(1),
	}, {
		// Just before 3am, power is still off.
		now: T(3).Add(-1),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Here: 1000,
			},
		},
		expectState: mkRelays(1),
	}, {
		// At 3am, the discretionary power relay
		// kicks in because we realise that it needs
		// to be on or it will miss its slot criteria.
		now: T(3),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Here: 1100,
			},
		},
		expectState: mkRelays(0, 1),
	}, {
		// It's still on just before 5am (the end of the slot).
		now: T(5).Add(-1),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Here: 1100,
			},
		},
		expectState: mkRelays(0, 1),
	}, {
		// ... and switches off at the end of the slot.
		now: T(5),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Here: 1100,
			},
		},
		expectState: mkRelays(1),
	}},
}, {
	about: "When several relays are discretionary, they turn on one at a time",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			// Relay 0 is on for at least 2 hours between 1am and 5am.
			Mode: hydroctl.InUse,
			InUse: []*hydroctl.Slot{{
				Start:        1 * time.Hour,
				SlotDuration: 4 * time.Hour,
				Kind:         hydroctl.Exactly,
				Duration:     2 * time.Hour,
			}},
		}, {
			// Relay 1 is the same as relay 0.
			Mode: hydroctl.InUse,
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
		// After MeterReactionDuration, the other relay
		// turns on.
		now:         T(1).Add(hydroctl.MeterReactionDuration),
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
		now:         T(3).Add(hydroctl.MeterReactionDuration),
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
		}},
	},
	assessNowTests: []assessNowTest{{
		// The relay comes on as usual at 1am.
		now: T(1),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 500,
			},
		},
		expectState: mkRelays(0),
		transition:  true,
	}, {
		// The meter updates to reflect the new use.
		now: T(1).Add(hydroctl.MeterReactionDuration),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 500,
				Here:      100,
			},
		},
		expectState: mkRelays(0),
	}, {
		// At 2am, the meter shows that we're importing
		// electricity, so the relay is switched off.
		now: T(2),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 500,
				Here:      1000,
			},
		},
		expectState: mkRelays(),
		transition:  true,
	}, {
		// At 3am, the generator starts generating enough
		// electricity again, so we switch the relay back on.
		now: T(3),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1100,
				Here:      1000,
			},
		},
		expectState: mkRelays(0),
		transition:  true,
	}, {
		// At 4am we've satisfied the slot requirements,
		// so we turn it off again.
		now: T(4),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1100,
				Here:      1100,
			},
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
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				// The generator is producing 3kW.
				Generated: 3000,
			},
		},
		expectState: mkRelays(0),
		transition:  true,
	}, {
		now: T(1).Add(hydroctl.MeterReactionDuration),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 3000,
				Here:      1000,
			},
		},
		expectState: mkRelays(0, 1),
		transition:  true,
	}, {
		now: T(1).Add(2 * hydroctl.MeterReactionDuration),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 3000,
				Here:      2000,
			},
		},
		expectState: mkRelays(0, 1, 2),
		transition:  true,
	}, {
		// A little while after, we're using all the generated
		// power but there's no problem with that.
		now: T(1).Add(2*hydroctl.MeterReactionDuration + time.Minute),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 3000,
				Here:      3000,
			},
		},
		expectState: mkRelays(0, 1, 2),
		transition:  true,
	}, {
		// At 2am, the other house starts using power,
		// so we switch off just enough relays to
		// hope that we stop using the excess power.
		now: T(2),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 3000,
				Here:      3000,
				Neighbour: 1500,
			},
		},
		expectState: mkRelays(2),
		transition:  true,
	}},
}, {
	about: "A sample case that failed",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode: hydroctl.NotInUse,
		}, {
			Mode: hydroctl.NotInUse,
		}, {
			Mode:     hydroctl.InUse,
			MaxPower: 5000,
			InUse: []*hydroctl.Slot{{
				Start:        13 * time.Hour,
				SlotDuration: 1 * time.Hour,
				Kind:         hydroctl.Exactly,
				Duration:     1 * time.Hour,
			}},
		}},
	},
	assessNowTests: []assessNowTest{{
		// At the time slot, the lowest-numbered relay comes on first.
		now:         T(13),
		expectState: mkRelays(2),
		transition:  true,
	}, {
		now:         T(13).Add(hydroctl.MinimumChangeDuration),
		expectState: mkRelays(2),
		transition:  true,
	}},
}, {
	about: "Given two discretionary relays that could be on and might start importing, we leave them off until forced",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode:     hydroctl.InUse,
			MaxPower: 1000,
			InUse: []*hydroctl.Slot{{
				Start:        10 * time.Hour,
				SlotDuration: time.Hour,
				Kind:         hydroctl.AtLeast,
				Duration:     5 * time.Minute,
			}},
		}, {
			Mode:     hydroctl.InUse,
			MaxPower: 1000,
			InUse: []*hydroctl.Slot{{
				Start:        10 * time.Hour,
				SlotDuration: time.Hour,
				Kind:         hydroctl.AtLeast,
				Duration:     5 * time.Minute,
			}},
		}},
	},
	assessNowTests: []assessNowTest{{
		// At the start of the slot, we don't switch either relay
		// on because either one might start importing.
		now: T(10).Add(0),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      700,
			},
		},
	}, {
		// It remains off...
		now: T(10).Add(5 * time.Minute),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      700,
			},
		},
	}, {
		// ... until there's no more time left in the slot, at which point
		// we need to switch 'em both on. The first relay is
		// preferred at this point because it's first numerically.
		now: T(11).Add(-5 * time.Minute),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      700,
			},
		},
		expectState: mkRelays(0),
		transition:  true,
	}, {
		// The second relay switches on after the usual delay.
		now: T(11).Add(-5 * time.Minute).Add(hydroctl.MinimumChangeDuration),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      1700,
			},
		},
		expectState: mkRelays(0, 1),
		transition:  true,
	}, {
		// Both remain on for their allotted 5 minutes.
		now: T(11).Add(-2 * time.Minute).Add(hydroctl.MinimumChangeDuration),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      2700,
			},
		},
		expectState: mkRelays(0, 1),
	}, {
		// And both switch off at the end of the slot.
		now: T(11),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      2700,
			},
		},
		transition: true,
	}},
}, {
	about: "Given three relays and only enough power for one of them, we'll cycle between them at CycleDuration frequency",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode:     hydroctl.InUse,
			MaxPower: 750,
			InUse: []*hydroctl.Slot{{
				Start:        10 * time.Hour,
				SlotDuration: time.Hour,
				Kind:         hydroctl.AtLeast,
				Duration:     23 * time.Minute,
			}},
		}, {
			Mode:     hydroctl.InUse,
			MaxPower: 800,
			InUse: []*hydroctl.Slot{{
				Start:        10 * time.Hour,
				SlotDuration: time.Hour,
				Kind:         hydroctl.AtLeast,
				Duration:     17 * time.Minute,
			}},
		}, {
			Mode:     hydroctl.InUse,
			MaxPower: 850,
			InUse: []*hydroctl.Slot{{
				Start:        10 * time.Hour,
				SlotDuration: time.Hour,
				Kind:         hydroctl.AtLeast,
				Duration:     17 * time.Minute,
			}},
		}},
	},
	assessNowTests: []assessNowTest{{
		// We switch a relay on to start with because we've got enough power to do that.
		now:         T(10).Add(0),
		expectState: mkRelays(0),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
			},
		},
		transition: true,
	}, {
		// We don't switch the second relay on because there's not enough spare
		// power to accommodate it.
		now:         T(10).Add(hydroctl.MeterReactionDuration),
		expectState: mkRelays(0),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      750,
			},
		},
	}, {
		// At the end of a cycle, the first relay turns off to
		// regain some power.
		now:         T(10).Add(hydroctl.CycleDuration),
		expectState: mkRelays(),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      750,
			},
		},
	}, {
		// When we see the power regained, we turn on the second meter.
		now:         T(10).Add(hydroctl.CycleDuration + hydroctl.MeterReactionDuration),
		expectState: mkRelays(1),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      0,
			},
		},
	}, {
		// It stays on.
		now:         T(10).Add(hydroctl.CycleDuration + 2*hydroctl.MeterReactionDuration),
		expectState: mkRelays(1),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      800,
			},
		},
	}, {
		// At the end of another cycle, the second meter turns off to
		// make more power available.
		now:         T(10).Add(2*hydroctl.CycleDuration + hydroctl.MeterReactionDuration),
		expectState: mkRelays(),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      800,
			},
		},
	}, {
		// When we see the power regained, we turn on the third meter.
		now:         T(10).Add(2*hydroctl.CycleDuration + 2*hydroctl.MeterReactionDuration),
		expectState: mkRelays(2),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      0,
			},
		},
	}, {
		// It stays on.
		now:         T(10).Add(2*hydroctl.CycleDuration + 3*hydroctl.MeterReactionDuration),
		expectState: mkRelays(2),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      850,
			},
		},
	}, {
		// At the end of another cycle, the third meter turns off to
		// make more power available.
		now:         T(10).Add(3*hydroctl.CycleDuration + 2*hydroctl.MeterReactionDuration),
		expectState: mkRelays(),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      800,
			},
		},
	}, {
		// When we see the power regained, we cycle back to the first meter again.
		now:         T(10).Add(3*hydroctl.CycleDuration + 3*hydroctl.MeterReactionDuration),
		expectState: mkRelays(0),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      0,
			},
		},
	}, {
		// It stays on.
		now:         T(10).Add(3*hydroctl.CycleDuration + 4*hydroctl.MeterReactionDuration),
		expectState: mkRelays(0),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      750,
			},
		},
	}},
}}

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
		var prevPowerUse hydroctl.PowerUseSample
		for j, innertest := range test.assessNowTests {
			c.Logf("\t%d. at %v", j, D(innertest.now))
			if innertest.transition {
				// Check just before the test time to make
				// sure the state is unchanged from the
				// previous test.
				if prevPowerUse.T0.IsZero() {
					// This will happen when there's a transition in the first step.
					prevPowerUse.T0 = innertest.now.Add(-1)
					prevPowerUse.T1 = innertest.now.Add(-1)
				}
				newState := hydroctl.Assess(hydroctl.AssessParams{
					Config:         &test.cfg,
					CurrentState:   state,
					History:        history,
					PowerUseSample: prevPowerUse,
					Logger:         clogger{c},
					Now:            innertest.now.Add(-1),
				})
				c.Assert(newState, gc.Equals, state, gc.Commentf("previous state"))
			}
			pu := innertest.powerUse
			if pu.T0.IsZero() {
				pu.T0 = innertest.now
			}
			if pu.T1.IsZero() {
				pu.T1 = innertest.now
			}
			state = hydroctl.Assess(hydroctl.AssessParams{
				Config:         &test.cfg,
				CurrentState:   state,
				History:        history,
				PowerUseSample: pu,
				Logger:         clogger{c},
				Now:            innertest.now,
			})
			c.Assert(state, gc.Equals, innertest.expectState)
			history.RecordState(state, innertest.now)
			c.Logf("new history: %v", &history)
			prevPowerUse = pu
		}
	}
}

var slotOverlapTests = []struct {
	about        string
	slot1, slot2 hydroctl.Slot
	expect       bool
}{{
	about: "exactly the same",
	slot1: hydroctl.Slot{
		Start:        time.Hour,
		SlotDuration: 2 * time.Hour,
	},
	slot2: hydroctl.Slot{
		Start:        time.Hour,
		SlotDuration: 2 * time.Hour,
	},
	expect: true,
}, {
	about: "one starts as the other finishes",
	slot1: hydroctl.Slot{
		Start:        time.Hour,
		SlotDuration: 2 * time.Hour,
	},
	slot2: hydroctl.Slot{
		Start:        3 * time.Hour,
		SlotDuration: time.Hour,
	},
	expect: false,
}, {
	about: "overlap by a second",
	slot1: hydroctl.Slot{
		Start:        time.Hour,
		SlotDuration: 2 * time.Hour,
	},
	slot2: hydroctl.Slot{
		Start:        3*time.Hour - time.Second,
		SlotDuration: time.Hour,
	},
	expect: true,
}, {
	about: "zero length slot within another one",
	slot1: hydroctl.Slot{
		Start: time.Hour,
	},
	slot2: hydroctl.Slot{
		Start:        0,
		SlotDuration: 2 * time.Hour,
	},
	expect: false,
}, {
	about:  "two zero-length slots",
	expect: false,
}}

func (suite) TestSlotOverlap(c *gc.C) {
	for i, test := range slotOverlapTests {
		c.Logf("test %d: %v", i, test.about)
		got := test.slot1.Overlaps(&test.slot2)
		c.Assert(got, gc.Equals, test.expect)
		// Try it reversed.
		got = test.slot2.Overlaps(&test.slot1)
		c.Assert(got, gc.Equals, test.expect)
	}
}

type clogger struct {
	c *gc.C
}

func (l clogger) Log(s string) {
	l.c.Logf("assess: %s", s)
}

func mkRelays(relays ...uint) hydroctl.RelayState {
	var state hydroctl.RelayState
	for _, r := range relays {
		state |= 1 << r
	}
	return state
}

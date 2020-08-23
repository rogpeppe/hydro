package hydroctl_test

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/rogpeppe/hydro/history"
	"github.com/rogpeppe/hydro/hydroctl"
)

var epoch = time.Date(2000, 01, 01, 0, 0, 0, 0, time.UTC)

var (
	ukTZ, _ = time.LoadLocation("Europe/London")
	// start and end dates of daylight savings time in the UK in 2019.
	dstStart = time.Date(2019, 03, 31, 0, 0, 0, 0, ukTZ)
	dstEnd   = time.Date(2019, 10, 27, 0, 0, 0, 0, ukTZ)
)

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
	testName        string
	previousUpdates []stateUpdate
	currentState    hydroctl.RelayState
	cfg             hydroctl.Config
	assessNowTests  []assessNowTest
}{{
	testName: "everything-off,-some-relays-that-are-always-on",
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
		now:         T(0).Add(hydroctl.DefaultMinimumChangeDuration),
		expectState: mkRelays(0, 5),
		transition:  true,
	}, {
		now:         T(1),
		expectState: mkRelays(0, 5),
	}},
}, {
	testName: "everything-on,-one-relay-that's-always-off",
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
	testName: "relay-on-for-exactly-2-hours-between-1am-and-5am",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode: hydroctl.InUse,
			InUse: []*hydroctl.Slot{{
				Start:    TD("01:00"),
				End:      TD("05:00"),
				Kind:     hydroctl.Exactly,
				Duration: 2 * time.Hour,
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
	testName: "relay-on-through-midnight",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode: hydroctl.InUse,
			InUse: []*hydroctl.Slot{{
				Start: TD("22:00"),
				End:   TD("02:00"),
				Kind:  hydroctl.Continuous,
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
	testName: "relay-on-for-at-least-2-hours-between-1am-and-5am",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode: hydroctl.InUse,
			InUse: []*hydroctl.Slot{{
				Start:    TD("01:00"),
				End:      TD("05:00"),
				Kind:     hydroctl.AtLeast,
				Duration: 2 * time.Hour,
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
	testName: "relay-on-for-at-most-2-hours-between-1am-and-5am",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode: hydroctl.InUse,
			InUse: []*hydroctl.Slot{{
				Start:    TD("01:00"),
				End:      TD("05:00"),
				Kind:     hydroctl.AtMost,
				Duration: 2 * time.Hour,
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
	testName: "when-lots-of-power-is-in-use,-discretionary-power-doesn't-kick-in-until-it-must",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			// Relay 0 is on for at least 2 hours between 1am and 5am.
			Mode:     hydroctl.InUse,
			MaxPower: 100,
			InUse: []*hydroctl.Slot{{
				Start:    TD("01:00"),
				End:      TD("05:00"),
				Kind:     hydroctl.AtLeast,
				Duration: 2 * time.Hour,
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
	testName: "When-several-relays-are-discretionary,-they-turn-on-one-at-a-time",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			// Relay 0 is on for at least 2 hours between 1am and 5am.
			Mode: hydroctl.InUse,
			InUse: []*hydroctl.Slot{{
				Start:    TD("01:00"),
				End:      TD("05:00"),
				Kind:     hydroctl.Exactly,
				Duration: 2 * time.Hour,
			}},
		}, {
			// Relay 1 is the same as relay 0.
			Mode: hydroctl.InUse,
			InUse: []*hydroctl.Slot{{
				Start:    TD("01:00"),
				End:      TD("05:00"),
				Kind:     hydroctl.Exactly,
				Duration: 2 * time.Hour,
			}},
		}},
	},
	assessNowTests: []assessNowTest{{
		// At 1am, relay 0 (arbitrary choice) turns on.
		now:         T(1),
		expectState: mkRelays(0),
		transition:  true,
	}, {
		// After DefaultMeterReactionDuration, the other relay
		// turns on.
		now:         T(1).Add(hydroctl.DefaultMeterReactionDuration),
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
		now:         T(3).Add(hydroctl.DefaultMeterReactionDuration),
		expectState: mkRelays(),
		transition:  true,
	}},
}, {
	testName: "When-a-discretionary-power-relay-is-on-and-there's-not-enough-power,-it-switches-off-until-there-is",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			// Relay 0 is on for at least 2 hours between 1am and 5am.
			Mode:     hydroctl.InUse,
			MaxPower: 100,
			InUse: []*hydroctl.Slot{{
				Start:    TD("01:00"),
				End:      TD("05:00"),
				Kind:     hydroctl.Exactly,
				Duration: 2 * time.Hour,
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
		now: T(1).Add(hydroctl.DefaultMeterReactionDuration),
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
	testName: "When-several-discretionary-power-relays-are-on-and-power-is-limited,-we-switch-enough-off-to-try-to-regain-the-power",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode:     hydroctl.InUse,
			MaxPower: 1000,
			InUse: []*hydroctl.Slot{{
				Start:    TD("01:00"),
				End:      TD("05:00"),
				Kind:     hydroctl.Exactly,
				Duration: 2 * time.Hour,
			}},
		}, {
			Mode:     hydroctl.InUse,
			MaxPower: 1000,
			InUse: []*hydroctl.Slot{{
				Start:    TD("01:00"),
				End:      TD("05:00"),
				Kind:     hydroctl.Exactly,
				Duration: 2 * time.Hour,
			}},
		}, {
			Mode:     hydroctl.InUse,
			MaxPower: 1000,
			InUse: []*hydroctl.Slot{{
				Start:    TD("01:00"),
				End:      TD("05:00"),
				Kind:     hydroctl.Exactly,
				Duration: 2 * time.Hour,
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
		now: T(1).Add(hydroctl.DefaultMeterReactionDuration),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 3000,
				Here:      1000,
			},
		},
		expectState: mkRelays(0, 1),
		transition:  true,
	}, {
		now: T(1).Add(2 * hydroctl.DefaultMeterReactionDuration),
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
		now: T(1).Add(2*hydroctl.DefaultMeterReactionDuration + time.Minute),
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
	testName: "A-sample-case-that-failed",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode: hydroctl.NotInUse,
		}, {
			Mode: hydroctl.NotInUse,
		}, {
			Mode:     hydroctl.InUse,
			MaxPower: 5000,
			InUse: []*hydroctl.Slot{{
				Start: TD("13:00"),
				End:   TD("14:00"),
				Kind:  hydroctl.Continuous,
			}},
		}},
	},
	assessNowTests: []assessNowTest{{
		// At the time slot, the lowest-numbered relay comes on first.
		now:         T(13),
		expectState: mkRelays(2),
		transition:  true,
	}, {
		now:         T(13).Add(hydroctl.DefaultMinimumChangeDuration),
		expectState: mkRelays(2),
		transition:  true,
	}},
}, {
	testName: "Given-two-discretionary-relays-that-could-be-on-and-might-start-importing,-we-leave-them-off-until-forced",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode:     hydroctl.InUse,
			MaxPower: 1000,
			InUse: []*hydroctl.Slot{{
				Start:    TD("10:00"),
				End:      TD("11:00"),
				Kind:     hydroctl.AtLeast,
				Duration: 5 * time.Minute,
			}},
		}, {
			Mode:     hydroctl.InUse,
			MaxPower: 1000,
			InUse: []*hydroctl.Slot{{
				Start:    TD("10:00"),
				End:      TD("11:00"),
				Kind:     hydroctl.AtLeast,
				Duration: 5 * time.Minute,
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
		now: T(11).Add(-5 * time.Minute).Add(hydroctl.DefaultMinimumChangeDuration),
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
		now: T(11).Add(-2 * time.Minute).Add(hydroctl.DefaultMinimumChangeDuration),
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
	testName: "Given-three-relays-and-only-enough-power-for-one-of-them,-we'll-cycle-between-them-at-DefaultCycleDuration-frequency",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{{
			Mode:     hydroctl.InUse,
			MaxPower: 750,
			InUse: []*hydroctl.Slot{{
				Start:    TD("10:00"),
				End:      TD("11:00"),
				Kind:     hydroctl.AtLeast,
				Duration: 23 * time.Minute,
			}},
		}, {
			Mode:     hydroctl.InUse,
			MaxPower: 800,
			InUse: []*hydroctl.Slot{{
				Start:    TD("10:00"),
				End:      TD("11:00"),
				Kind:     hydroctl.AtLeast,
				Duration: 17 * time.Minute,
			}},
		}, {
			Mode:     hydroctl.InUse,
			MaxPower: 850,
			InUse: []*hydroctl.Slot{{
				Start:    TD("10:00"),
				End:      TD("11:00"),
				Kind:     hydroctl.AtLeast,
				Duration: 17 * time.Minute,
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
		now:         T(10).Add(hydroctl.DefaultMeterReactionDuration),
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
		now:         T(10).Add(hydroctl.DefaultCycleDuration),
		expectState: mkRelays(),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      750,
			},
		},
	}, {
		// When we see the power regained, we turn on the second meter.
		now:         T(10).Add(hydroctl.DefaultCycleDuration + hydroctl.DefaultMeterReactionDuration),
		expectState: mkRelays(1),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      0,
			},
		},
	}, {
		// It stays on.
		now:         T(10).Add(hydroctl.DefaultCycleDuration + 2*hydroctl.DefaultMeterReactionDuration),
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
		now:         T(10).Add(2*hydroctl.DefaultCycleDuration + hydroctl.DefaultMeterReactionDuration),
		expectState: mkRelays(),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      800,
			},
		},
	}, {
		// When we see the power regained, we turn on the third meter.
		now:         T(10).Add(2*hydroctl.DefaultCycleDuration + 2*hydroctl.DefaultMeterReactionDuration),
		expectState: mkRelays(2),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      0,
			},
		},
	}, {
		// It stays on.
		now:         T(10).Add(2*hydroctl.DefaultCycleDuration + 3*hydroctl.DefaultMeterReactionDuration),
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
		now:         T(10).Add(3*hydroctl.DefaultCycleDuration + 2*hydroctl.DefaultMeterReactionDuration),
		expectState: mkRelays(),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      800,
			},
		},
	}, {
		// When we see the power regained, we cycle back to the first meter again.
		now:         T(10).Add(3*hydroctl.DefaultCycleDuration + 3*hydroctl.DefaultMeterReactionDuration),
		expectState: mkRelays(0),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      0,
			},
		},
	}, {
		// It stays on.
		now:         T(10).Add(3*hydroctl.DefaultCycleDuration + 4*hydroctl.DefaultMeterReactionDuration),
		expectState: mkRelays(0),
		powerUse: hydroctl.PowerUseSample{
			PowerUse: hydroctl.PowerUse{
				Generated: 1000,
				Here:      750,
			},
		},
	}},
}, {
	testName: "daylight-savings-time-starts",
	// When DST starts (at 1am), an hour is lost.
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{
			0: {
				Mode:     hydroctl.InUse,
				MaxPower: 100,
				InUse: []*hydroctl.Slot{{
					Start: TD("00:05"),
					End:   TD("03:00"),
					Kind:  hydroctl.Continuous,
				}},
			},
		},
	},
	currentState: mkRelays(),
	assessNowTests: []assessNowTest{{
		now:         dstStart,
		expectState: mkRelays(),
	}, {
		now:         dstStart.Add(5 * time.Minute),
		transition:  true,
		expectState: mkRelays(0),
	}, {
		now:         dstStart.Add(2 * time.Hour),
		transition:  true,
		expectState: mkRelays(),
	}},
}, {
	testName: "transition-time-non-existent",
	// Check that things still work OK if the transition time doesn't
	// actually happen because DST start skipped it. Go's time.Date
	// behaviour gives a time an hour later whereas arguably for this
	// purpose we'd want the slot to start on the next available time.
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{
			0: {
				Mode:     hydroctl.InUse,
				MaxPower: 100,
				InUse: []*hydroctl.Slot{{
					Start: TD("01:30"),
					End:   TD("03:00"),
					Kind:  hydroctl.Continuous,
				}},
			},
		},
	},
	currentState: mkRelays(),
	assessNowTests: []assessNowTest{{
		now:         dstStart,
		expectState: mkRelays(),
	}, {
		now:         dstStart.Add(time.Hour + 30*time.Minute),
		transition:  true,
		expectState: mkRelays(0),
	}, {
		now:         dstStart.Add(2 * time.Hour),
		transition:  true,
		expectState: mkRelays(),
	}},
}, {
	testName: "duration-longer-than-slot-because-of-DST-transition",
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{
			0: {
				Mode:     hydroctl.InUse,
				MaxPower: 100,
				InUse: []*hydroctl.Slot{{
					Start:    TD("01:30"),
					End:      TD("03:00"),
					Kind:     hydroctl.AtLeast,
					Duration: time.Hour,
				}},
			},
		},
	},
	currentState: mkRelays(),
	assessNowTests: []assessNowTest{{
		now:         dstStart,
		expectState: mkRelays(),
	}, {
		now:         dstStart.Add(time.Hour + 30*time.Minute),
		transition:  true,
		expectState: mkRelays(0),
	}, {
		// Even though we haven't been able to give it the full
		// amount of time, the relay should still switch off at the
		// end of the slot.g
		now:         dstStart.Add(2 * time.Hour),
		transition:  true,
		expectState: mkRelays(),
	}},
}, {
	testName: "daylight-savings-time-ends",
	// When DST ends (at 1am), an hour is gained.
	cfg: hydroctl.Config{
		Relays: []hydroctl.RelayConfig{
			0: {
				Mode:     hydroctl.InUse,
				MaxPower: 100,
				InUse: []*hydroctl.Slot{{
					Start: TD("00:05"),
					End:   TD("03:00"),
					Kind:  hydroctl.Continuous,
				}},
			},
		},
	},
	currentState: mkRelays(),
	assessNowTests: []assessNowTest{{
		now:         dstEnd,
		expectState: mkRelays(),
	}, {
		now:         dstEnd.Add(5 * time.Minute),
		transition:  true,
		expectState: mkRelays(0),
	}, {
		now:         dstEnd.Add(4 * time.Hour),
		transition:  true,
		expectState: mkRelays(),
	}},
}}

func TestAssess(t *testing.T) {
	c := qt.New(t)
	for _, test := range assessTests {
		c.Run(test.testName, func(c *qt.C) {
			state := test.currentState

			history, err := history.New(&history.MemStore{})
			c.Assert(err, qt.IsNil)
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
					c.Assert(newState, qt.Equals, state, qt.Commentf("previous state"))
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
				c.Assert(state, qt.Equals, innertest.expectState)
				history.RecordState(state, innertest.now)
				c.Logf("new history: %v", &history)
				prevPowerUse = pu
			}
		})
	}
}

var slotOverlapTests = []struct {
	testName     string
	slot1, slot2 hydroctl.Slot
	expect       bool
}{{
	testName: "exactly-the-same",
	slot1: hydroctl.Slot{
		Start: TD("01:00"),
		End:   TD("03:00"),
	},
	slot2: hydroctl.Slot{
		Start: TD("01:00"),
		End:   TD("03:00"),
	},
	expect: true,
}, {
	testName: "one-starts-as-the-other-finishes",
	slot1: hydroctl.Slot{
		Start: TD("01:00"),
		End:   TD("03:00"),
	},
	slot2: hydroctl.Slot{
		Start: TD("03:00"),
		End:   TD("04:00"),
	},
	expect: false,
}, {
	testName: "overlap-by-a-minute",
	slot1: hydroctl.Slot{
		Start: TD("01:00"),
		End:   TD("03:00"),
	},
	slot2: hydroctl.Slot{
		Start: TD("02:59"),
		End:   TD("03:59"),
	},
	expect: true,
}, {
	testName: "24-hour-slot-within-another-one",
	slot1: hydroctl.Slot{
		Start: TD("01:00"),
		End:   TD("01:00"),
	},
	slot2: hydroctl.Slot{
		Start: TD("00:00"),
		End:   TD("02:00"),
	},
	expect: true,
}, {
	testName: "two 24 hour slots",
	expect:   true,
}}

func TestSlotOverlap(t *testing.T) {
	c := qt.New(t)
	for _, test := range slotOverlapTests {
		c.Run(test.testName, func(c *qt.C) {
			got := test.slot1.Overlaps(&test.slot2)
			c.Assert(got, qt.Equals, test.expect)
			// Try it reversed.
			got = test.slot2.Overlaps(&test.slot1)
			c.Assert(got, qt.Equals, test.expect)
		})
	}
}

type clogger struct {
	c *qt.C
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

func TD(s string) hydroctl.TimeOfDay {
	td, err := hydroctl.ParseTimeOfDay(s)
	if err != nil {
		panic(err)
	}
	return td
}

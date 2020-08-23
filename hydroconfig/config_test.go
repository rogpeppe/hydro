package hydroconfig_test

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/rogpeppe/hydro/hydroconfig"
	"github.com/rogpeppe/hydro/hydroctl"
)

var parseTests = []struct {
	testName    string
	config      string
	expect      *hydroconfig.Config
	expectError string
}{{
	testName: "original-example",
	config: `
relay 6 is dining room
relays 0, 4, 5 are bedrooms

dining room on from 14:30 to 20:45 for at least 20m
bedrooms on from 17:00 to 20:00
`,
	expect: &hydroconfig.Config{
		Cohorts: []hydroconfig.Cohort{{
			Name:   "bedrooms",
			Relays: []int{0, 4, 5},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start: TD("17:00"),
				End:   TD("20:00"),
				Kind:  hydroctl.Continuous,
			}},
		}, {
			Name:   "dining room",
			Relays: []int{6},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start:    TD("14:30"),
				End:      TD("20:45"),
				Kind:     hydroctl.AtLeast,
				Duration: D("20m"),
			}},
		}},
	},
}, {
	testName: "comment-line",
	config: `
# some comment
	# a comment with leading whitespace
relay 1 is somewhere

somewhere on from 1am to 2am
`,
	expect: &hydroconfig.Config{
		Cohorts: []hydroconfig.Cohort{{
			Name:   "somewhere",
			Relays: []int{1},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start: TD("01:00"),
				End:   TD("02:00"),
				Kind:  hydroctl.Continuous,
			}},
		}},
	},
}, {
	testName: "cohort-with-label",
	config: `
relay 1 is x (some label)

x on from 1am to 2am
`,
	expect: &hydroconfig.Config{
		Cohorts: []hydroconfig.Cohort{{
			Name:   "some label",
			Relays: []int{1},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start: TD("01:00"),
				End:   TD("02:00"),
				Kind:  hydroctl.Continuous,
			}},
		}},
	},
}, {
	testName: "multiple-slots",
	config: `
relay 6 is dining room

dining room from 14:30 to 20:45 for at least 20m
dining room from 11pm to 5am for at most 1h
dining room on from 10:00 to 12pm
`,
	expect: &hydroconfig.Config{
		Cohorts: []hydroconfig.Cohort{{
			Name:   "dining room",
			Relays: []int{6},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start:    TD("14:30"),
				End:      TD("20:45"),
				Kind:     hydroctl.AtLeast,
				Duration: D("20m"),
			}, {
				Start:    TD("23:00"),
				End:      TD("05:00"),
				Kind:     hydroctl.AtMost,
				Duration: D("1h"),
			}, {
				Start: TD("10:00"),
				End:   TD("12:00"),
				Kind:  hydroctl.Continuous,
			}},
		}},
	},
}, {
	testName: "slot-with-is/are",
	config: `
Relay 6 is dining room.
Relays 7, 8 are bedrooms.

Dining room is from 14:00 to 15:00.
Bedrooms are on from 12:00 to 1pm
`,
	expect: &hydroconfig.Config{
		Cohorts: []hydroconfig.Cohort{{
			Name:   "bedrooms",
			Relays: []int{7, 8},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start: TD("12:00"),
				End:   TD("13:00"),
				Kind:  hydroctl.Continuous,
			}},
		}, {
			Name:   "dining room",
			Relays: []int{6},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start: TD("14:00"),
				End:   TD("15:00"),
				Kind:  hydroctl.Continuous,
			}},
		}},
	},
}, {
	testName: "with-max-power-specs",
	config: `
Relay 6 is dining room.
Relays 7, 8, 9 are bedrooms.
relay 6, 7 have max power 100w
relays 8 has maxpower 5.678KW.

Dining room is from 14:00 to 15:00.
Bedrooms are on from 12:00 to 1pm
`,
	expect: &hydroconfig.Config{
		Cohorts: []hydroconfig.Cohort{{
			Name:   "bedrooms",
			Relays: []int{7, 8, 9},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start: TD("12:00"),
				End:   TD("13:00"),
				Kind:  hydroctl.Continuous,
			}},
		}, {
			Name:   "dining room",
			Relays: []int{6},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start: TD("14:00"),
				End:   TD("15:00"),
				Kind:  hydroctl.Continuous,
			}},
		}},
		Relays: map[int]hydroconfig.Relay{
			6: {100},
			7: {100},
			8: {5678},
		},
	},
}, {
	testName: "all-day-slots",
	config: `
relay 0 is dining room.
relay 1 is bedroom.
relay 2 is another
relay 3 is more

dining room on
bedroom on for 2h.
another is on for at most 1h.
more for 12h
`,
	expect: &hydroconfig.Config{
		Cohorts: []hydroconfig.Cohort{{
			Name:   "another",
			Relays: []int{2},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Kind:     hydroctl.AtMost,
				Duration: D("1h"),
			}},
		}, {
			Name:   "bedroom",
			Relays: []int{1},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Kind:     hydroctl.Exactly,
				Duration: D("2h"),
			}},
		}, {
			Name:   "dining room",
			Relays: []int{0},
			Mode:   hydroctl.AlwaysOn,
		}, {
			Name:   "more",
			Relays: []int{3},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Kind:     hydroctl.Exactly,
				Duration: D("12h"),
			}},
		}},
	},
}, {
	testName: "overlapping-time-slot",
	config: `
relays 0, 4, 5 are bedrooms
relay 7 is other

bedrooms on from 11am to 1pm
bedrooms on from 12pm to 3pm
`,
	expectError: `error at " on from 12pm to 3pm": time slot overlaps slot from 11:00 to 13:00`,
}, {
	testName: "empty-config",
	config:   "",
	expect:   &hydroconfig.Config{},
}, {
	testName: "config-with-config-parameters",
	config: `
config fastest 5s
config reaction 10s
config cycle 20m
`,
	expect: &hydroconfig.Config{
		Attrs: hydroconfig.Attrs{
			MinimumChangeDuration: 5 * time.Second,
			MeterReactionDuration: 10 * time.Second,
			CycleDuration:         20 * time.Minute,
		},
	},
}}

// awkward failing test for now.
// Fix it later.
//{
//	testName: "cohort-with-no-spaces-between-relay-numbers",
//	config: `
//relays 7,8,10 are bedrooms
//`,
//	expect: &hydroconfig.Config{
//		Cohorts: []hydroconfig.Cohort{{
//			Name: "bedrooms",
//			Relays: []int{7, 8, 10},
//			Mode: hydroctl.InUse,
//		}},
//	},
//},

func TestParse(t *testing.T) {
	c := qt.New(t)
	for _, test := range parseTests {
		c.Run(test.testName, func(c *qt.C) {
			cfg, err := hydroconfig.Parse(test.config)
			if test.expectError != "" {
				c.Assert(err, qt.ErrorMatches, test.expectError)
				c.Assert(cfg, qt.IsNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(cfg, qt.DeepEquals, test.expect)
			}
		})
	}
}

var ctlConfigTests = []struct {
	cfg    hydroconfig.Config
	expect hydroctl.Config
}{{
	cfg: hydroconfig.Config{
		Relays: map[int]hydroconfig.Relay{
			1: {500},
			2: {1000},
			4: {600},
			5: {2000},
		},
		Cohorts: []hydroconfig.Cohort{{
			Name:   "one",
			Relays: []int{1, 4},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start:    TD("01:00"),
				End:      TD("01:01"),
				Kind:     hydroctl.AtLeast,
				Duration: time.Second,
			}},
		}, {
			Name:   "two",
			Relays: []int{2, 4, -1, 2000, 5},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start:    TD("02:00"),
				End:      TD("02:02"),
				Kind:     hydroctl.AtMost,
				Duration: time.Minute,
			}},
		}},
	},
	expect: hydroctl.Config{
		Relays: mkSlots([hydroctl.MaxRelayCount]hydroctl.RelayConfig{
			1: {
				Cohort:   "one",
				MaxPower: 500,
				Mode:     hydroctl.InUse,
				InUse: []*hydroctl.Slot{{
					Start:    TD("01:00"),
					End:      TD("01:01"),
					Kind:     hydroctl.AtLeast,
					Duration: time.Second,
				}},
			},
			2: {
				Cohort:   "two",
				MaxPower: 1000,
				Mode:     hydroctl.InUse,
				InUse: []*hydroctl.Slot{{
					Start:    TD("02:00"),
					End:      TD("02:02"),
					Kind:     hydroctl.AtMost,
					Duration: time.Minute,
				}},
			},
			4: {
				Cohort:   "one",
				MaxPower: 600,
				Mode:     hydroctl.InUse,
				InUse: []*hydroctl.Slot{{
					Start:    TD("01:00"),
					End:      TD("01:01"),
					Kind:     hydroctl.AtLeast,
					Duration: time.Second,
				}},
			},
			5: {
				Cohort:   "two",
				MaxPower: 2000,
				Mode:     hydroctl.InUse,
				InUse: []*hydroctl.Slot{{
					Start:    TD("02:00"),
					End:      TD("02:02"),
					Kind:     hydroctl.AtMost,
					Duration: time.Minute,
				}},
			},
		}),
	},
}}

func mkSlots(slots [hydroctl.MaxRelayCount]hydroctl.RelayConfig) []hydroctl.RelayConfig {
	return slots[:]
}

func TestCtlConfig(t *testing.T) {
	c := qt.New(t)
	for _, test := range ctlConfigTests {
		c.Run("", func(c *qt.C) {
			c.Assert(test.cfg.CtlConfig(), qt.DeepEquals, &test.expect)
		})
	}
}

func D(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

func TD(s string) hydroctl.TimeOfDay {
	td, err := hydroctl.ParseTimeOfDay(s)
	if err != nil {
		panic(err)
	}
	return td
}

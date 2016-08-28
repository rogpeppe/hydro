package hydroconfig_test

import (
	"time"

	jc "github.com/juju/testing/checkers"
	"github.com/rogpeppe/hydro/hydroconfig"
	"github.com/rogpeppe/hydro/hydroctl"

	gc "gopkg.in/check.v1"
)

var _ = gc.Suite(&configSuite{})

type configSuite struct{}

var parseTests = []struct {
	about       string
	config      string
	expect      *hydroconfig.Config
	expectError *hydroconfig.ConfigParseError
}{{
	about: "original example",
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
				Start:        D("17h"),
				SlotDuration: D("3h"),
				Kind:         hydroctl.Exactly,
				Duration:     D("3h"),
			}},
		}, {
			Name:   "dining room",
			Relays: []int{6},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start:        D("14h30m"),
				SlotDuration: D("6h15m"),
				Kind:         hydroctl.AtLeast,
				Duration:     D("20m"),
			}},
		}},
	},
}, {
	about: "multiple slots",
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
				Start:        D("14h30m"),
				SlotDuration: D("6h15m"),
				Kind:         hydroctl.AtLeast,
				Duration:     D("20m"),
			}, {
				Start:        D("23h"),
				SlotDuration: D("6h"),
				Kind:         hydroctl.AtMost,
				Duration:     D("1h"),
			}, {
				Start:        D("10h"),
				SlotDuration: D("2h"),
				Kind:         hydroctl.Exactly,
				Duration:     D("2h"),
			}},
		}},
	},
}, {
	about: "slot with is/are",
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
				Start:        D("12h"),
				SlotDuration: D("1h"),
				Kind:         hydroctl.Exactly,
				Duration:     D("1h"),
			}},
		}, {
			Name:   "dining room",
			Relays: []int{6},
			Mode:   hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start:        D("14h"),
				SlotDuration: D("1h"),
				Kind:         hydroctl.Exactly,
				Duration:     D("1h"),
			}},
		}},
	},
}, {
	about:  "empty config",
	config: "",
	expect: &hydroconfig.Config{},
}}

// awkward failing test for now.
// Fix it later.
//{
//	about: "cohort with no spaces between relay numbers",
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

func (*configSuite) TestParse(c *gc.C) {
	for i, test := range parseTests {
		c.Logf("test %d; %s", i, test.about)
		cfg, err := hydroconfig.Parse(test.config)
		if test.expectError != nil {
			c.Assert(err, jc.DeepEquals, test.expectError)
			c.Assert(cfg, gc.IsNil)
		} else {
			c.Assert(err, gc.IsNil)
			c.Assert(cfg, jc.DeepEquals, test.expect)
		}
	}
}

var ctlConfigTests = []struct {
	cfg    hydroconfig.Config
	expect hydroctl.Config
}{{
	cfg: hydroconfig.Config{
		Cohorts: []hydroconfig.Cohort{{
			Name:     "one",
			Relays:   []int{1, 4},
			MaxPower: 500,
			Mode:     hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start:        time.Hour,
				SlotDuration: time.Minute,
				Kind:         hydroctl.AtLeast,
				Duration:     time.Second,
			}},
		}, {
			Name:     "two",
			Relays:   []int{2, 4, -1, 2000, 5},
			MaxPower: 1000,
			Mode:     hydroctl.InUse,
			InUseSlots: []*hydroctl.Slot{{
				Start:        2 * time.Hour,
				SlotDuration: 2 * time.Minute,
				Kind:         hydroctl.AtMost,
				Duration:     time.Minute,
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
					Start:        time.Hour,
					SlotDuration: time.Minute,
					Kind:         hydroctl.AtLeast,
					Duration:     time.Second,
				}},
			},
			2: {
				Cohort:   "two",
				MaxPower: 1000,
				Mode:     hydroctl.InUse,
				InUse: []*hydroctl.Slot{{
					Start:        2 * time.Hour,
					SlotDuration: 2 * time.Minute,
					Kind:         hydroctl.AtMost,
					Duration:     time.Minute,
				}},
			},
			4: {
				Cohort:   "one",
				MaxPower: 500,
				Mode:     hydroctl.InUse,
				InUse: []*hydroctl.Slot{{
					Start:        time.Hour,
					SlotDuration: time.Minute,
					Kind:         hydroctl.AtLeast,
					Duration:     time.Second,
				}},
			},
			5: {
				Cohort:   "two",
				MaxPower: 1000,
				Mode:     hydroctl.InUse,
				InUse: []*hydroctl.Slot{{
					Start:        2 * time.Hour,
					SlotDuration: 2 * time.Minute,
					Kind:         hydroctl.AtMost,
					Duration:     time.Minute,
				}},
			},
		}),
	},
}}

func mkSlots(slots [hydroctl.MaxRelayCount]hydroctl.RelayConfig) []hydroctl.RelayConfig {
	return slots[:]
}

func (*configSuite) TestCtlConfig(c *gc.C) {
	for i, test := range ctlConfigTests {
		c.Logf("test %d", i)
		c.Assert(test.cfg.CtlConfig(), jc.DeepEquals, &test.expect)
	}
}

func D(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

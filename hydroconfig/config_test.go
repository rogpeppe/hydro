package hydroserver

import (
	"time"

	jc "github.com/juju/testing/checkers"
	"github.com/rogpeppe/hydro/hydroctl"

	gc "gopkg.in/check.v1"
)

var _ = gc.Suite(&configSuite{})

type configSuite struct{}

var parseTests = []struct {
	about       string
	config      string
	expect      *Config
	expectError *ConfigParseError
}{{
	about: "original example",
	config: `
relay 6 is dining room
relays 0, 4, 5 are bedrooms

dining room on from 14:30 to 20:45 for at least 20m
bedrooms on from 17:00 to 20:00
`,
	expect: &Config{
		Cohorts: []Cohort{{
			Name:   "bedrooms",
			Relays: []int{0, 4, 5},
			InUseSlots: []hydroctl.Slot{{
				Start:        D("17h"),
				SlotDuration: D("3h"),
				Kind:         hydroctl.Exactly,
				Duration:     D("3h"),
			}},
		}, {
			Name:   "dining room",
			Relays: []int{6},
			InUseSlots: []hydroctl.Slot{{
				Start:        D("14h30m"),
				SlotDuration: D("6h15m"),
				Kind:         hydroctl.AtLeast,
				Duration:     D("20m"),
			}},
		}},
	},
}, {
	about: "multple slots",
	config: `
relay 6 is dining room

dining room from 14:30 to 20:45 for at least 20m
dining room from 11pm to 5am for at most 1h
dining room on from 10:00 to 12pm
`,
	expect: &Config{
		Cohorts: []Cohort{{
			Name:   "dining room",
			Relays: []int{6},
			InUseSlots: []hydroctl.Slot{{
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
}}

func (*configSuite) TestParse(c *gc.C) {
	for i, test := range parseTests {
		c.Logf("test %d; %s", i, test.about)
		cfg, err := Parse(test.config)
		if test.expectError != nil {
			c.Assert(err, jc.DeepEquals, test.expectError)
			c.Assert(cfg, gc.IsNil)
		} else {
			c.Assert(err, gc.IsNil)
			c.Assert(cfg, jc.DeepEquals, test.expect)
		}
	}
}

func D(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

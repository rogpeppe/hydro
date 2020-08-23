package hydroctl_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/rogpeppe/hydro/hydroctl"
)

var chargeablePowerTests = []struct {
	testName string
	use      hydroctl.PowerUse
	expect   hydroctl.PowerChargeable
}{{
	testName: "zero-gets-zero",
}, {
	testName: "no-one-using-anything,-all-gets-exported-to-grid",
	use: hydroctl.PowerUse{
		Generated: 50,
	},
	expect: hydroctl.PowerChargeable{
		ExportGrid: 50,
	},
}, {
	testName: "within-generated-power,-it's-all-exported",
	use: hydroctl.PowerUse{
		Generated: 50,
		Neighbour: 7,
		Here:      5,
	},
	expect: hydroctl.PowerChargeable{
		ExportGrid:      50 - (5 + 7),
		ExportNeighbour: 7,
		ExportHere:      5,
	},
}, {
	testName: "if-here-uses-excess-power,-they-pay",
	use: hydroctl.PowerUse{
		Generated: 50,
		Neighbour: 20,
		Here:      40,
	},
	expect: hydroctl.PowerChargeable{
		ExportNeighbour: 20,
		ExportHere:      30,
		ImportHere:      10,
	},
}, {
	testName: "if-neighbour-uses-excess-power,-they-pay",
	use: hydroctl.PowerUse{
		Generated: 50,
		Neighbour: 40,
		Here:      20,
	},
	expect: hydroctl.PowerChargeable{
		ExportNeighbour: 30,
		ExportHere:      20,
		ImportNeighbour: 10,
	},
}, {
	testName: "if-both-use-excess-power,-each-pay-for-import-proportionally-to-their-relative-usage",
	use: hydroctl.PowerUse{
		Generated: 50,
		Here:      60,
		Neighbour: 55,
	},
	expect: hydroctl.PowerChargeable{
		ExportNeighbour: 25,
		ExportHere:      25,
		ImportNeighbour: (60 + 55 - 50) * (55.0 / (55 + 60)),
		ImportHere:      (60 + 55 - 50) * (60.0 / (55 + 60)),
	},
}}

func TestChargeablePower(t *testing.T) {
	c := qt.New(t)
	for _, test := range chargeablePowerTests {
		c.Run(test.testName, func(c *qt.C) {
			pc := hydroctl.ChargeablePower(test.use)
			assertEqual(c, "ExportGrid", pc.ExportGrid, test.expect.ExportGrid)
			assertEqual(c, "ExportNeighbour", pc.ExportNeighbour, test.expect.ExportNeighbour)
			assertEqual(c, "ExportHere here", pc.ExportHere, test.expect.ExportHere)
			assertEqual(c, "ImportNeighbour", pc.ImportNeighbour, test.expect.ImportNeighbour)
			assertEqual(c, "ImportHere", pc.ImportHere, test.expect.ImportHere)
			// Check invariant: all the power used should be accounted for.
			totalExported := pc.ExportGrid + pc.ExportNeighbour + pc.ExportHere
			assertEqual(c, "total exported", totalExported, test.use.Generated)
			// Check invariant: when importing, the power imported should be what's used less what's generated.
			if imported := test.use.Here + test.use.Neighbour - test.use.Generated; imported > 0 {
				assertEqual(c, "total imported", pc.ImportNeighbour+pc.ImportHere, imported)
			}
		})
	}
}

const eps = 0.0001

func assertEqual(c *qt.C, what string, got, want float64) {
	if got < want-eps || got > want+eps {
		c.Errorf("unexpected value for %v, got %v want %v", what, got, want)
	}
}

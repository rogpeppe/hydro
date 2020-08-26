package meterstat

import (
	"io"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var approxDeepEquals = qt.CmpEquals(cmpopts.EquateApprox(0, 0.001))

var usageReaderTests = []struct {
	testName    string
	samples     string
	start       time.Time
	quantum     time.Duration
	expectError string
	expect      []float64
	expectTotal Usage
}{{
	testName: "allSamples",
	samples: `
946814400000,1000
946814410000,1010
946814415000,1030
`[1:],
	start:   epoch,
	quantum: time.Second,
	expect:  []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 4, 4, 4, 4, 4},
	expectTotal: Usage{
		Energy:  30,
		Samples: 2,
	},
}, {
	testName: "startLater",
	samples: `
946814400000,1000
946814410000,1010
946814415000,1030
`[1:],
	start:   epoch.Add(3 * time.Second),
	quantum: time.Second,
	expect:  []float64{1, 1, 1, 1, 1, 1, 1, 4, 4, 4, 4, 4},
	expectTotal: Usage{
		Energy:  27,
		Samples: 1 + 7.0/10,
	},
}, {
	testName: "startTooEarly",
	samples: `
946814400000,1000
946814410000,1010
`[1:],
	start:       epoch.Add(-time.Second),
	quantum:     time.Second,
	expectError: `no sample found before the start time \(earliest sample 2000-01-02 12:00:00 \+0000 GMT; start time 2000-01-02 11:59:59 \+0000 GMT\)`,
}, {
	testName: "noSamples",
	samples:  ``,
	start:    epoch,
	quantum:  time.Second,
}, {
	testName: "oneSample",
	samples:  `946814400000,1000`,
	start:    epoch,
	quantum:  time.Second,
}}

func TestUsageReader(t *testing.T) {
	c := qt.New(t)
	for _, test := range usageReaderTests {
		c.Run(test.testName, func(c *qt.C) {
			r := NewUsageReader(
				NewSampleReader(strings.NewReader(test.samples)),
				test.start,
				test.quantum,
			)
			c.Assert(r.Quantum(), qt.Equals, test.quantum)
			// We test the Energy values because it's too tedious to test
			// all the Sample values at the moment...
			var samples []float64
			var total Usage
			foundError := false
			t := test.start
			for {
				c.Assert(r.Time(), qt.DeepEquals, t)
				sample, err := r.ReadUsage()
				if err == io.EOF {
					break
				}
				t = t.Add(test.quantum)
				if err != nil {
					if test.expectError != "" {
						foundError = true
						c.Assert(err, qt.ErrorMatches, test.expectError)
						break
					}
					c.Fatalf("error calling ReadUsage: %v", err)
				}
				samples = append(samples, sample.Energy)
				total = total.Add(sample)
			}
			if test.expectError != "" && !foundError {
				c.Errorf("no error found; want %q", test.expectError)
			}
			c.Assert(samples, approxDeepEquals, test.expect)
			c.Assert(total, approxDeepEquals, test.expectTotal)
		})
	}
}

func TestSumUsage(t *testing.T) {
	c := qt.New(t)
	r0 := NewUsageReader(
		NewMemSampleReader([]Sample{{
			Time:        epoch,
			TotalEnergy: 1000,
		}, {
			Time:        epoch.Add(10 * time.Second),
			TotalEnergy: 2000,
		}, {
			Time:        epoch.Add(20 * time.Second),
			TotalEnergy: 6000,
		}}),
		epoch,
		time.Second,
	)
	r1 := NewUsageReader(
		NewMemSampleReader([]Sample{{
			Time:        epoch,
			TotalEnergy: 100,
		}, {
			Time:        epoch.Add(5 * time.Second),
			TotalEnergy: 200,
		}, {
			Time:        epoch.Add(8 * time.Second),
			TotalEnergy: 220,
		}, {
			Time:        epoch.Add(20 * time.Second),
			TotalEnergy: 1000,
		}}),
		epoch,
		time.Second,
	)
	r2 := NewUsageReader(
		NewMemSampleReader([]Sample{{
			Time:        epoch,
			TotalEnergy: 10,
		}, {
			Time:        epoch.Add(2 * time.Second),
			TotalEnergy: 20,
		}, {
			Time:        epoch.Add(20 * time.Second),
			TotalEnergy: 30,
		}}),
		epoch,
		time.Second,
	)
	ur := SumUsage(r0, r1, r2)
	var sum Usage
	var usages []Usage
	c.Assert(ur.Quantum(), qt.Equals, time.Second)
	for {
		c.Assert(ur.Time(), qt.DeepEquals, epoch.Add(time.Second*time.Duration(len(usages))))
		u, err := ur.ReadUsage()
		if err == io.EOF {
			break
		}
		c.Assert(err, qt.IsNil)
		usages = append(usages, u)
		sum = sum.Add(u)
	}
	c.Check(usages, approxDeepEquals, []Usage{
		{125, .8},
		{125, .8},
		{120.55555555555556, .3556},
		{120.55555555555556, .3556},
		{120.55555555555556, .3556},
		{107.22222222222221, .4889},
		{107.22222222222224, .4889},
		{107.22222222222221, .4889},
		{165.55555555555554, .2389},
		{165.55555555555554, .2389},
		{465.55555555555554, .2389},
		{465.55555555555554, .2389},
		{465.55555555555554, .2389},
		{465.55555555555554, .2389},
		{465.55555555555554, .2389},
		{465.55555555555554, .2389},
		{465.55555555555554, .2389},
		{465.55555555555554, .2389},
		{465.55555555555554, .2389},
		{465.55555555555554, .2389},
	})
	// Check that the total energy sums correctly to the difference in total energy between the
	// start and end of all the sample sets.
	c.Check(sum, approxDeepEquals, Usage{
		Energy: 0.0 +
			6000 - 1000 +
			1000 - 100 +
			30 - 10,
		// Note: the number of samples is the total number of samples less the
		// number sample sources, because the last sample from each source
		// is not counted.
		Samples: 7,
	})
}

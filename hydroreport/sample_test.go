package hydroreport

import (
	"io"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var approxDeepEquals = qt.CmpEquals(cmpopts.EquateApprox(0, 0.001))

var epoch = time.Unix(946814400, 0) // 2000-01-02 12:00:00Z

func TestSampleReader(t *testing.T) {
	c := qt.New(t)
	r := NewSampleReader(strings.NewReader(`
946814400000,1000
946814410005,1010
946814415000,23456
`[1:]))
	samples, err := readAll(r)
	c.Assert(err, qt.IsNil)

	c.Assert(samples, qt.DeepEquals, []Sample{{
		Time:        epoch,
		TotalEnergy: 1000,
	}, {
		Time:        epoch.Add(10*time.Second + 5*time.Millisecond),
		TotalEnergy: 1010,
	}, {
		Time:        epoch.Add(15 * time.Second),
		TotalEnergy: 23456,
	}})
}

func TestMultiReader(t *testing.T) {
	c := qt.New(t)
	r0 := NewSampleReader(strings.NewReader(`
946814400000,1000
946814410005,1010
946814415000,23456
`[1:]))
	r1 := NewMemSampleReader([]Sample{{
		Time:        epoch.Add(30 * time.Second),
		TotalEnergy: 3000,
	}, {
		Time:        epoch.Add(36 * time.Second),
		TotalEnergy: 4000,
	}})
	samples, err := readAll(MultiSampleReader(r0, r1))
	c.Assert(err, qt.IsNil)
	c.Assert(samples, qt.DeepEquals, []Sample{{
		Time:        epoch,
		TotalEnergy: 1000,
	}, {
		Time:        epoch.Add(10*time.Second + 5*time.Millisecond),
		TotalEnergy: 1010,
	}, {
		Time:        epoch.Add(15 * time.Second),
		TotalEnergy: 23456,
	}, {
		Time:        epoch.Add(30 * time.Second),
		TotalEnergy: 3000,
	}, {
		Time:        epoch.Add(36 * time.Second),
		TotalEnergy: 4000,
	}})
}

var usageReaderTests = []struct {
	testName    string
	samples     string
	start       time.Time
	quantum     time.Duration
	expectError string
	expect      []float64
	expectTotal float64
}{{
	testName: "allSamples",
	samples: `
946814400000,1000
946814410000,1010
946814415000,1030
`[1:],
	start:       epoch,
	quantum:     time.Second,
	expect:      []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 4, 4, 4, 4, 4},
	expectTotal: 30,
}, {
	testName: "startLater",
	samples: `
946814400000,1000
946814410000,1010
946814415000,1030
`[1:],
	start:       epoch.Add(3 * time.Second),
	quantum:     time.Second,
	expect:      []float64{1, 1, 1, 1, 1, 1, 1, 4, 4, 4, 4, 4},
	expectTotal: 27,
}, {
	testName: "startLater",
	samples: `
946814400000,1000
946814410000,1010
946814415000,1030
`[1:],
	start:       epoch.Add(3 * time.Second),
	quantum:     time.Second,
	expect:      []float64{1, 1, 1, 1, 1, 1, 1, 4, 4, 4, 4, 4},
	expectTotal: 27,
}, {
	testName: "startTooEarly",
	samples: `
946814400000,1000
946814410000,1010
`[1:],
	start:       epoch.Add(-time.Second),
	quantum:     time.Second,
	expectError: "no sample found before the start time",
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
			var samples []float64
			total := float64(0)
			foundError := false
			for {
				sample, err := r.ReadUsage()
				if err == io.EOF {
					break
				}
				if err != nil {
					if test.expectError != "" {
						foundError = true
						c.Assert(err, qt.ErrorMatches, test.expectError)
						break
					}
					c.Fatalf("error calling ReadUsage: %v", err)
				}
				samples = append(samples, sample)
				total += sample
			}
			if test.expectError != "" && !foundError {
				c.Errorf("no error found; want %q", test.expectError)
			}
			c.Assert(samples, approxDeepEquals, test.expect)
			c.Assert(total, approxDeepEquals, test.expectTotal)
		})
	}
}

func readAll(r SampleReader) ([]Sample, error) {
	var samples []Sample
	for {
		s, err := r.ReadSample()
		if err != nil {
			if err == io.EOF {
				return samples, nil
			}
			return samples, err
		}
		samples = append(samples, s)
	}
}

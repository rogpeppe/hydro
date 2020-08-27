package meterstat

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

var overlapsTests = []struct {
	testName string
	// TODO use TimeRange here.
	at0, at1, bt0, bt1 time.Time
	expectOverlaps     bool
	expectIntersect    TimeRange
}{{
	testName:        "both-empty-same",
	at0:             epoch,
	at1:             epoch,
	bt0:             epoch,
	bt1:             epoch,
	expectOverlaps:  true,
	expectIntersect: TimeRange{epoch, epoch},
}, {
	testName:        "both-empty-different",
	at0:             epoch,
	at1:             epoch,
	bt0:             epoch.Add(1),
	bt1:             epoch.Add(1),
	expectOverlaps:  false,
	expectIntersect: TimeRange{},
}, {
	testName:        "a-encloses-b",
	at0:             epoch,
	at1:             epoch.Add(time.Hour),
	bt0:             epoch.Add(time.Minute),
	bt1:             epoch.Add(2 * time.Minute),
	expectOverlaps:  true,
	expectIntersect: TimeRange{epoch.Add(time.Minute), epoch.Add(2 * time.Minute)},
}, {
	testName:        "b-encloses-a",
	at0:             epoch.Add(time.Minute),
	at1:             epoch.Add(2 * time.Minute),
	bt0:             epoch,
	bt1:             epoch.Add(time.Hour),
	expectOverlaps:  true,
	expectIntersect: TimeRange{epoch.Add(time.Minute), epoch.Add(2 * time.Minute)},
}, {
	testName:        "a-encloses-empty-b",
	at0:             epoch,
	at1:             epoch.Add(time.Hour),
	bt0:             epoch.Add(time.Minute),
	bt1:             epoch.Add(time.Minute),
	expectOverlaps:  true,
	expectIntersect: TimeRange{epoch.Add(time.Minute), epoch.Add(time.Minute)},
}, {
	testName:        "b-encloses-empty-a",
	at0:             epoch.Add(time.Minute),
	at1:             epoch.Add(time.Minute),
	bt0:             epoch,
	bt1:             epoch.Add(time.Hour),
	expectOverlaps:  true,
	expectIntersect: TimeRange{epoch.Add(time.Minute), epoch.Add(time.Minute)},
}, {
	testName:        "a-overlaps-b-start",
	at0:             epoch.Add(-time.Minute),
	at1:             epoch.Add(time.Minute),
	bt0:             epoch,
	bt1:             epoch.Add(time.Hour),
	expectOverlaps:  true,
	expectIntersect: TimeRange{epoch, epoch.Add(time.Minute)},
}, {
	testName:        "a-overlaps-b-end",
	at0:             epoch.Add(time.Hour - time.Minute),
	at1:             epoch.Add(time.Hour + time.Minute),
	bt0:             epoch,
	bt1:             epoch.Add(time.Hour),
	expectOverlaps:  true,
	expectIntersect: TimeRange{epoch.Add(time.Hour - time.Minute), epoch.Add(time.Hour)},
}, {
	testName:       "a-beyond-b",
	at0:            epoch.Add(time.Hour + time.Minute),
	at1:            epoch.Add(time.Hour + 2*time.Minute),
	bt0:            epoch,
	bt1:            epoch.Add(time.Hour),
	expectOverlaps: false,
}, {
	testName:       "b-beyond-a",
	at0:            epoch,
	at1:            epoch.Add(time.Hour),
	bt0:            epoch.Add(time.Hour + time.Minute),
	bt1:            epoch.Add(time.Hour + 2*time.Minute),
	expectOverlaps: false,
}, {
	testName:        "a-exactly-at-end",
	at0:             epoch,
	at1:             epoch.Add(time.Minute),
	bt0:             epoch.Add(time.Minute),
	bt1:             epoch.Add(2 * time.Minute),
	expectOverlaps:  true,
	expectIntersect: TimeRange{epoch.Add(time.Minute), epoch.Add(time.Minute)},
}}

func TestTimeOverlaps(t *testing.T) {
	c := qt.New(t)
	for _, test := range overlapsTests {
		c.Run(test.testName, func(c *qt.C) {
			t0 := TimeRange{test.at0, test.at1}
			t1 := TimeRange{test.bt0, test.bt1}
			c.Assert(t0.Overlaps(t1), qt.Equals, test.expectOverlaps)
			c.Assert(t0.Intersect(t1), qt.Equals, test.expectIntersect)
		})
	}
}

var timeRangeConstrainTests = []struct {
	r      TimeRange
	d      time.Duration
	expect TimeRange
}{{
	r:      TimeRange{epoch, epoch},
	d:      time.Hour,
	expect: TimeRange{epoch, epoch},
}, {
	r:      TimeRange{epoch, epoch.Add(time.Hour + time.Minute)},
	d:      time.Hour,
	expect: TimeRange{epoch, epoch.Add(time.Hour)},
}, {
	r:      TimeRange{epoch, epoch.Add(time.Minute)},
	d:      time.Hour,
	expect: TimeRange{epoch, epoch},
}, {
	r: TimeRange{epoch.Add(time.Minute), epoch.Add(2 * time.Minute)},
	d: time.Hour,
}}

func TestTimeRangeConstrain(t *testing.T) {
	c := qt.New(t)
	for _, test := range timeRangeConstrainTests {
		c.Run("", func(c *qt.C) {
			c.Assert(test.r.Constrain(test.d), qt.DeepEquals, test.expect)
		})
	}
}

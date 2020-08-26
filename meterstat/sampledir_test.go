package meterstat

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

// TODO tests for MeterSampleDir

var overlapsTests = []struct {
	testName           string
	at0, at1, bt0, bt1 time.Time
	expect             bool
}{{
	testName: "both-empty-same",
	at0:      epoch,
	at1:      epoch,
	bt0:      epoch,
	bt1:      epoch,
	expect:   true,
}, {
	testName: "both-empty-different",
	at0:      epoch,
	at1:      epoch,
	bt0:      epoch.Add(1),
	bt1:      epoch.Add(1),
	expect:   false,
}, {
	testName: "a-encloses-b",
	at0:      epoch,
	at1:      epoch.Add(time.Hour),
	bt0:      epoch.Add(time.Minute),
	bt1:      epoch.Add(2 * time.Minute),
	expect:   true,
}, {
	testName: "b-encloses-a",
	at0:      epoch.Add(time.Minute),
	at1:      epoch.Add(2 * time.Minute),
	bt0:      epoch,
	bt1:      epoch.Add(time.Hour),
	expect:   true,
}, {
	testName: "a-encloses-empty-b",
	at0:      epoch,
	at1:      epoch.Add(time.Hour),
	bt0:      epoch.Add(time.Minute),
	bt1:      epoch.Add(time.Minute),
	expect:   true,
}, {
	testName: "b-encloses-empty-a",
	at0:      epoch.Add(time.Minute),
	at1:      epoch.Add(time.Minute),
	bt0:      epoch,
	bt1:      epoch.Add(time.Hour),
	expect:   true,
}, {
	testName: "a-overlaps-b-start",
	at0:      epoch.Add(-time.Minute),
	at1:      epoch.Add(time.Minute),
	bt0:      epoch,
	bt1:      epoch.Add(time.Hour),
	expect:   true,
}, {
	testName: "a-overlaps-b-end",
	at0:      epoch.Add(time.Hour - time.Minute),
	at1:      epoch.Add(time.Hour + time.Minute),
	bt0:      epoch,
	bt1:      epoch.Add(time.Hour),
	expect:   true,
}, {
	testName: "a-beyond-b",
	at0:      epoch.Add(time.Hour + time.Minute),
	at1:      epoch.Add(time.Hour + 2*time.Minute),
	bt0:      epoch,
	bt1:      epoch.Add(time.Hour),
	expect:   false,
}, {
	testName: "b-beyond-a",
	at0:      epoch,
	at1:      epoch.Add(time.Hour),
	bt0:      epoch.Add(time.Hour + time.Minute),
	bt1:      epoch.Add(time.Hour + 2*time.Minute),
	expect:   false,
}, {
	testName: "a-exactly-at-end",
	at0:      epoch,
	at1:      epoch.Add(time.Minute),
	bt0:      epoch.Add(time.Minute),
	bt1:      epoch.Add(2 * time.Minute),
	expect:   true,
}}

func TestTimeOverlaps(t *testing.T) {
	c := qt.New(t)
	for _, test := range overlapsTests {
		c.Run(test.testName, func(c *qt.C) {
			c.Assert(timeOverlaps(test.at0, test.at1, test.bt0, test.bt1), qt.Equals, test.expect)
		})
	}
}

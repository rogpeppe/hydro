package meterstat

import (
	"fmt"
	"time"
)

type TimeRange struct {
	T0, T1 time.Time
}

func (r0 TimeRange) Overlaps(r1 TimeRange) bool {
	r0.mustCanonical()
	r1.mustCanonical()
	return !r0.T0.After(r1.T1) && !r1.T0.After(r0.T1)
}

func (r TimeRange) Canon() TimeRange {
	if r.T1.Before(r.T0) {
		r.T0, r.T1 = r.T1, r.T0
	}
	return r
}

func (r TimeRange) Equal(r1 TimeRange) bool {
	return r.T0.Equal(r1.T0) && r.T1.Equal(r1.T1)
}

func (r TimeRange) String() string {
	return fmt.Sprintf("[%v %v]", r.T0, r.T1)
}

// Constrain returns r constrained so that its start and end
// points lie on a multiple of d. If the range overlaps a multiple
// of d, the rreturned range is within r, although it might be empty.
//
// If the range does not overlap a multiple of d, the returned
// range will be zero.
func (r TimeRange) Constrain(d time.Duration) TimeRange {
	r1 := TimeRange{
		T0: r.T0.Add(d - 1).Truncate(d),
		T1: r.T1.Truncate(d),
	}
	if r1.T1.Before(r.T0) {
		// No multiple within the range.
		return TimeRange{}
	}
	if r1.T1.Before(r1.T0) {
		r1.T1 = r1.T0
	}
	return r1
}

// Intersect returns the largest time range overlapped by
// both r0 and r1. If the two ranges don't overlap then
// the zero time range will be returned.
func (r0 TimeRange) Intersect(r1 TimeRange) TimeRange {
	if !r0.Overlaps(r1) {
		return TimeRange{}
	}
	if r0.T0.Before(r1.T0) {
		r0.T0 = r1.T0
	}
	if r0.T1.After(r1.T1) {
		r0.T1 = r1.T1
	}
	return r0
}

func (r TimeRange) mustCanonical() {
	if r.T1.Before(r.T0) {
		panic(fmt.Errorf("out-of-order time interval [%v %v]", r.T0, r.T1))
	}
}

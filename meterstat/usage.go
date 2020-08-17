package meterstat

import (
	"fmt"
	"time"
)

// UsageReader produces a sequence of energy usage values at regular
// intervals from a point sample source
type UsageReader interface {
	// ReadUsage reads the energy used in the previous quantum of time
	// and advances to the next quantum interval.
	// It returns io.EOF when there are no more samples available.
	ReadUsage() (float64, error)

	// Time returns the start of the interval that the next ReadUsage
	// call will return the usage from. It increments by the interval
	// quantum each time ReadUsage is called.
	Time() time.Time

	// Quantum returns the interval quantum. It always returns the
	// same value for a given UsageReader.
	Quantum() time.Duration
}

type usageReader struct {
	r SampleReader
	// quantum holds the sampling interval.
	quantum time.Duration
	// err holds the (persistent) last error.
	err error
	// started holds whether we've located the initial samples.
	started bool
	// prevEnergy holds the total energy at the previous sample.
	prevEnergy float64
	// current holds the time which we're about to return a sample for.
	current time.Time
	// s0 and s1 hold the two closest known samples to current.
	s0, s1 Sample
}

// NewUsageReader uses samples read from r to construct a quantized view of the
// energy usage. The returned UsageReader will produce samples starting at quantum
// past the given start time and at quantum intervals subsequently, each holding the
// energy used from the beginning of that quantum until its end.
//
// The SampleReader r must hold samples that monotonically increase over time
// and include at least one sample that's not after the start time.
func NewUsageReader(r SampleReader, start time.Time, quantum time.Duration) UsageReader {
	if quantum == 0 {
		panic("zero quantum")
	}
	return &usageReader{
		r:       r,
		current: start,
		quantum: quantum,
	}
}

func (r *usageReader) Time() time.Time {
	if !r.started {
		return r.current
	}
	return r.current.Add(-r.quantum)
}

func (r *usageReader) Quantum() time.Duration {
	return r.quantum
}

// ReadUsage reads the energy used in the previous quantum of time
// and advances to the next quantum interval.
func (r *usageReader) ReadUsage() (float64, error) {
	if err := r.init(); err != nil {
		return 0, err
	}
	if r.current.After(r.s1.Time) {
		// We've gone beyond the extent of the current sample,
		// so acquire another pair of samples.
		if err := r.acquireSamples(); err != nil {
			r.err = err
			return 0, r.err
		}
	}
	// We've already got samples sufficient for this quantum, so use them.
	currentEnergy := r.energyAt(r.current)
	s := currentEnergy - r.prevEnergy
	r.prevEnergy = currentEnergy
	r.current = r.current.Add(r.quantum)
	return s, nil
}

// init acquires the first pair of samples that tell us the
// initial energy reading.
func (r *usageReader) init() error {
	if r.started {
		return r.err
	}
	if err := r.acquireSamples(); err != nil {
		r.err = err
		return err
	}
	if r.s0.Time.After(r.current) {
		r.err = fmt.Errorf("no sample found before the start time (earliest sample %v; start time %v)", r.s0.Time, r.current)
		return r.err
	}
	// Initialize the energy reading for the start of the period.
	r.prevEnergy = r.energyAt(r.current)
	r.current = r.current.Add(r.quantum)
	r.started = true
	return nil
}

// acquireSamples acquires two samples that closest bracket r.current.
func (r *usageReader) acquireSamples() error {
	r.s0 = r.s1
	for {
		sample, err := r.r.ReadSample()
		if err != nil {
			return err
		}
		if !sample.Time.After(r.s0.Time) {
			// A sample that isn't strictly monotonically increasing. Ignore it.
			// TODO print warning?
			continue
		}
		if !sample.Time.Before(r.current) {
			// We've found a sample that's after or equal to the current
			// time, so as we're sure that samples monotonically increase,
			// we've also found the closest previous sample.
			r.s1 = sample
			if r.s0.Time.IsZero() {
				// We're getting the first sample and it's exactly at the
				// start time. In this case, it's OK for the two samples to be
				// identical.
				r.s0 = sample
			}
			return nil
		}
		r.s0 = sample
	}
}

// energyAt returns the interpolated energy reading at the given
// time, which must be between r.s0.Time and r.s1.Time.
func (r *usageReader) energyAt(t time.Time) float64 {
	if t.Before(r.s0.Time) || t.After(r.s1.Time) {
		panic("time out of bounds")
	}
	if r.s0.Time.Equal(r.s1.Time) {
		// We're being asked for the energy at the exact instant
		// that both samples are for. This can happen for the very
		// first sample.
		return r.s1.TotalEnergy
	}
	sdt := r.s1.Time.Sub(r.s0.Time)
	sde := r.s1.TotalEnergy - r.s0.TotalEnergy
	dt := t.Sub(r.s0.Time)
	return float64(sde)/float64(sdt)*float64(dt) + r.s0.TotalEnergy
}

// SumUsage sums the usage readings from all the given readers.
// It panics if any of the given readers start at different times or have different quantum
// interval values.
//
// The reader will stop returning samples when any of the given
// readers stop returning samples.
// TODO would it be better to continue while there are samples
// from any reader?
func SumUsage(rs ...UsageReader) UsageReader {
	if err := checkUsageReaderConsistency(rs...); err != nil {
		panic(err)
	}
	return &sumUsageReader{
		readers: rs,
	}
}

func checkUsageReaderConsistency(rs ...UsageReader) error {
	if len(rs) == 0 {
		return fmt.Errorf("no UsageReaders provided")
	}
	startTime := rs[0].Time()
	quantum := rs[0].Quantum()
	for _, r := range rs {
		if !r.Time().Equal(startTime) {
			return fmt.Errorf("inconsistent start time")
		}
		if r.Quantum() != quantum {
			return fmt.Errorf("inconsistent quantum")
		}
	}
	return nil
}

type sumUsageReader struct {
	err     error
	readers []UsageReader
}

func (ur *sumUsageReader) Time() time.Time {
	return ur.readers[0].Time()
}

func (ur *sumUsageReader) Quantum() time.Duration {
	return ur.readers[0].Quantum()
}

func (ur *sumUsageReader) ReadUsage() (float64, error) {
	if ur.err != nil {
		return 0, ur.err
	}
	sum := 0.0
	for _, r := range ur.readers {
		usage, err := r.ReadUsage()
		if err != nil {
			ur.err = err
			return 0, err
		}
		sum += usage
	}
	return sum, nil
}

package hydroreport

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Sample represents an energy sample reading from a meter.
type Sample struct {
	Time        time.Time
	TotalEnergy float64
}

// NewSampleReader returns a SampleReader that reads samples from
// a textual sample file. Each line consists of three comma-separated fields:
// 	timestamp of sample (in milliseconds since the unix epoch)
//	total energy generated so far (in WH).
func NewSampleReader(r io.Reader) *SampleReader {
	return &SampleReader{
		scanner: bufio.NewScanner(r),
	}
}

type SampleReader struct {
	scanner *bufio.Scanner
}

func (r *SampleReader) ReadSample() (Sample, error) {
	if !r.scanner.Scan() {
		if r.scanner.Err() == nil {
			return Sample{}, io.EOF
		}
		return Sample{}, r.scanner.Err()
	}
	fields := strings.Split(r.scanner.Text(), ",")
	if len(fields) != 2 {
		return Sample{}, fmt.Errorf("invalid sample line found: %q", r.scanner.Text())
	}
	ts, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return Sample{}, fmt.Errorf("invalid timestamp in sample line %q", fields[0])
	}
	energy, err := strconv.ParseFloat(fields[1], 64)
	if err != nil {
		return Sample{}, fmt.Errorf("invalid energy value in sample line %q", fields[1])
	}
	return Sample{
		Time:        time.Unix(int64(ts/1000), (int64(ts)%1000)*1e6),
		TotalEnergy: energy,
	}, nil
}

type UsageReader struct {
	r *SampleReader
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
func NewUsageReader(r *SampleReader, start time.Time, quantum time.Duration) *UsageReader {
	if quantum == 0 {
		panic("zero quantum")
	}
	return &UsageReader{
		r:       r,
		current: start,
		quantum: quantum,
	}
}

// ReadUsage reads the energy used in the previous quantum of time
// and advances to the next quantum interval.
func (r *UsageReader) ReadUsage() (float64, error) {
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
func (r *UsageReader) init() error {
	if r.started {
		return r.err
	}
	if err := r.acquireSamples(); err != nil {
		r.err = err
		return err
	}
	if r.s0.Time.After(r.current) {
		r.err = fmt.Errorf("no sample found before the start time")
		return r.err
	}
	// Initialize the energy reading for the start of the period.
	r.prevEnergy = r.energyAt(r.current)
	r.current = r.current.Add(r.quantum)
	r.started = true
	return nil
}

// acquireSamples acquires two samples that closest bracket r.current.
func (r *UsageReader) acquireSamples() error {
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
func (r *UsageReader) energyAt(t time.Time) float64 {
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

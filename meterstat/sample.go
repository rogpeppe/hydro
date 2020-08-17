package meterstat

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
	// Time holds the time that the sample was taken.
	Time time.Time
	// TotalEnergy holds the total energy generated up until the sample was
	// taken, in WH.
	TotalEnergy float64
}

// SampleReader represents a source of point samples.
// Each call to ReadSample returns the next sample in the stream.
// Successive amples should hold monotonically increasing Time
// and TotalEnergy values.
type SampleReader interface {
	// ReadSample returns the next sample in the stream.
	// It returns io.EOF at the end of the available samples.
	ReadSample() (Sample, error)
}

// NewMemSampleReader returns a SampleReader that returns
// successive values from the given slice.
func NewMemSampleReader(samples []Sample) SampleReader {
	return &memSampleReader{samples}
}

type memSampleReader struct {
	samples []Sample
}

func (r *memSampleReader) ReadSample() (Sample, error) {
	if len(r.samples) == 0 {
		return Sample{}, io.EOF
	}
	s := r.samples[0]
	r.samples = r.samples[1:]
	return s, nil
}

// MultiSampleReader returns a SampleReader that returns samples
// from all the given readers, earliest samples first.
// It ensures that the total energy samples are monontonically
// increasing, discarding samples that don't.
func MultiSampleReader(rs ...SampleReader) SampleReader {
	return &multiReader{
		readers: rs,
		samples: make([]Sample, len(rs)),
	}
}

type multiReader struct {
	err     error
	readers []SampleReader
	samples []Sample
	prev    Sample
}

func (r *multiReader) ReadSample() (Sample, error) {
	for {
		s, err := r.readSample()
		if err != nil {
			return Sample{}, err
		}
		if s.TotalEnergy < r.prev.TotalEnergy || !s.Time.After(r.prev.Time) {
			// It's not monotonically increasing so discard it.
			continue
		}
		r.prev = s
		return s, nil
	}
}

// readSample is like ReadSample except that it doesn't discard
// samples that aren't monotonic.
func (r *multiReader) readSample() (Sample, error) {
	if r.err != nil {
		return Sample{}, r.err
	}
	// First make sure we've got at least one sample
	// from each reader.
	// Note: don't use range because we mutate the slice
	// in the loop.
	for i := 0; i < len(r.samples); i++ {
		s := &r.samples[i]
		if !s.Time.IsZero() {
			continue
		}
		s1, err := r.readers[i].ReadSample()
		if err == nil {
			r.samples[i] = s1
			continue
		}
		if err == io.EOF {
			// reader terminated. remove it from the slice.
			copy(r.readers[i:], r.readers[i+1:])
			r.readers[len(r.readers)-1] = nil
			r.readers = r.readers[0 : len(r.readers)-1]
			copy(r.samples[i:], r.samples[i+1:])
			r.samples = r.samples[0 : len(r.samples)-1]
			continue
		}
		// TODO we might want this to be more resilient to errors
		// and just ignore them?
		r.err = err
		return Sample{}, r.err
	}
	if len(r.readers) == 0 {
		return Sample{}, io.EOF
	}
	// Find the sample with the smallest time.
	minSampleIndex := 0
	for i := 1; i < len(r.samples); i++ {
		s := &r.samples[i]
		if s.Time.Before(r.samples[minSampleIndex].Time) {
			minSampleIndex = i
		}
	}
	s := r.samples[minSampleIndex]
	r.samples[minSampleIndex] = Sample{}
	return s, nil
}

// NewSampleReader returns a SampleReader that reads samples from
// a textual sample file. Each line consists of three comma-separated fields:
// 	timestamp of sample (in milliseconds since the unix epoch)
//	total energy generated so far (in WH).
func NewSampleReader(r io.Reader) SampleReader {
	return &fileSampleReader{
		scanner: bufio.NewScanner(r),
	}
}

type fileSampleReader struct {
	scanner *bufio.Scanner
}

func (r *fileSampleReader) ReadSample() (Sample, error) {
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

// WriteSamples reads all the samples from r and writes them to w
// in the format understood by NewSampleReader.
func WriteSamples(w io.Writer, r SampleReader) error {
	for {
		s, err := r.ReadSample()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("error reading sample: %v", err)
		}
		if err := WriteSample(w, s); err != nil {
			return fmt.Errorf("error writing sample: %v", err)
		}
	}
}

// WriteSample writes a single sample to w in the format understood by NewSampleReader.
func WriteSample(w io.Writer, s Sample) error {
	_, err := fmt.Fprintf(w, "%d,%.0f\n", s.Time.UnixNano() / 1e6, s.TotalEnergy)
	return err
}

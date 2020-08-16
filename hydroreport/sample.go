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
// from all the given readers in sequence.
func MultiSampleReader(rs ...SampleReader) SampleReader {
	return &multiReader{
		readers: rs,
	}
}

type multiReader struct {
	err     error
	readers []SampleReader
}

func (r *multiReader) ReadSample() (Sample, error) {
	if r.err != nil {
		return Sample{}, r.err
	}
	for {
		if len(r.readers) == 0 {
			r.err = io.EOF
			return Sample{}, r.err
		}
		s, err := r.readers[0].ReadSample()
		if err == nil {
			return s, nil
		}
		if err != io.EOF {
			r.err = err
			return Sample{}, r.err
		}
		r.readers = r.readers[1:]
	}
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

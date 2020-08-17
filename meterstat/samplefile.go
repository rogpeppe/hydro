package meterstat

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"
)

// SampleFileTimeRange returns the times of the oldest and newest samples
// in the sample file at the given path, assuming that all sample times in the file
// are monotonically increasing.
func SampleFileTimeRange(path string) (t0, t1 time.Time, err error) {
	f, err := os.Open(path)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	defer f.Close()
	r := NewSampleReader(f)
	s0, err := r.ReadSample()
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("cannot read initial sample: %v", err)
	}
	info, err := f.Stat()
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("cannot get file info: %v", err)
	}
	// Read the last part of the file to find the final line.
	const maxLineLen = 50 // overkill - it's just an int and a float.
	if info.Size() > maxLineLen {
		_, err := f.Seek(-maxLineLen, io.SeekEnd)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("cannot seek to end of file: %v", err)
		}
	} else {
		_, err := f.Seek(0, io.SeekStart)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("cannot seek to start of file: %v", err)
		}
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("cannot read last part of sample file: %v", err)
	}
	i := bytes.LastIndexByte(data, '\n')
	if i != -1 && i < len(data)-1 {
		// The file doesn't end with a newline, so it probably ends with an invalid record,
		// so ignore it.
		data = data[0 : i+1]
	}
	i = bytes.LastIndexByte(data[0:len(data)-1], '\n')
	if i == -1 {
		// There's only one line in the file, which means that there's
		// only one sample, so just return that.
		return s0.Time, s0.Time, nil
	}
	r = NewSampleReader(bytes.NewReader(data[i+1:]))
	s1, err := r.ReadSample()
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("cannot read final sample: %v", err)
	}
	return s0.Time, s1.Time, nil
}

// OpenSampleFile returns a SampleReader implementation that
// reads samples from the given file. The file is only kept open
// after the second sample has been read, which means that many
// SampleFile instances can be kept open without keeping their
// respective files open (for example to pass to MultiSampleReader).
//
// The SampleFile should be closed after use.
func OpenSampleFile(path string) (*SampleFile, error) {
	// Open the file, read the first sample from it, then close it.
	// This means we'll be able to open many sample files at once
	// without hitting open file limits.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s, err := NewSampleReader(f).ReadSample()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("cannot read first sample from %q: %v", path, err)
	}
	if err == nil && s.Time.IsZero() {
		// A valid sample should never have the zero time.
		return nil, fmt.Errorf("sample has zero time")
	}
	return &SampleFile{
		path:        path,
		firstSample: s,
	}, nil
}

type SampleFile struct {
	doneFirst   bool
	firstSample Sample
	closed      bool
	path        string
	r           SampleReader
	f           *os.File
}

func (sf *SampleFile) ReadSample() (Sample, error) {
	if sf.closed {
		return Sample{}, fmt.Errorf("sample file %q: read after close", sf.path)
	}
	if !sf.doneFirst {
		sf.doneFirst = true
		if sf.firstSample.Time.IsZero() {
			return Sample{}, io.EOF
		}
		return sf.firstSample, nil
	}
	if sf.r == nil {
		f, err := os.Open(sf.path)
		if err != nil {
			return Sample{}, err
		}
		sf.f = f
		sf.r = NewSampleReader(f)
		// Read and discard the first sample that we've already returned.
		_, err = sf.r.ReadSample()
		if err != nil {
			return Sample{}, fmt.Errorf("cannot discard first sample from %q: %v", sf.path, err)
		}
	}
	s, err := sf.r.ReadSample()
	if err == nil {
		return s, nil
	}
	if err == io.EOF {
		// Close the file but don't actually mark the reader as closed
		// so we free up the file descriptor.
		sf.Close()
		sf.closed = false
		// Ensure that subsequent reads continue to return io.EOF
		// rather than starting again at the beginning of the file.
		sf.r = eofReader{}
		return Sample{}, io.EOF
	}
	return Sample{}, fmt.Errorf("cannot read sample from %q: %v", sf.path, err)
}

// Close closes the SampleFile.
func (sf *SampleFile) Close() error {
	var err error
	if sf.f != nil {
		err = sf.f.Close()
		sf.f = nil
		sf.r = nil
	}
	sf.closed = true
	return err
}

type eofReader struct{}

func (eofReader) ReadSample() (Sample, error) {
	return Sample{}, io.EOF
}

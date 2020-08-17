package meterstat

import (
	"fmt"
	"io"
	"os"
)

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

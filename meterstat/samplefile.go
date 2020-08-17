package meterstat

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

func OpenSampleFile(path string) (*SampleFile, error) {
	info, err := SampleFileInfo(path)
	if err != nil {
		return nil, err
	}
	return info.Open(), nil
}

// OpenSampleFile returns information on a sample file.
//
// Empty sample files are considered to be invalid - an error
// will be returned if there are no samples in the file.
func SampleFileInfo(path string) (*FileInfo, error) {
	// Open the file, read the first sample from it, then close it.
	// This means we'll be able to open many sample files at once
	// without hitting open file limits.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s0, err := NewSampleReader(f).ReadSample()
	if err != nil {
		if err == io.EOF {
			err = fmt.Errorf("no samples in file")
		}
		return nil, fmt.Errorf("cannot read first sample from %q: %v", path, err)
	}
	if err == nil && s0.Time.IsZero() {
		// A valid sample should never have the zero time.
		return nil, fmt.Errorf("sample has zero time")
	}
	s1, err := readLastSample(f)
	if err != nil {
		return nil, err
	}
	return &FileInfo{
		path:        path,
		firstSample: s0,
		lastSample:  s1,
	}, nil
}

type FileInfo struct {
	firstSample Sample
	lastSample  Sample
	path        string
}

// Path returns the path to the sample file.
func (info *FileInfo) Path() string {
	return info.path
}

// FirstSample returns the first sample in the file.
func (info *FileInfo) FirstSample() Sample {
	return info.firstSample
}

// LastSample returns the last sample in the file. It always returns
// the sample that was last when the file was first opened - ignoring
// anything added after that.
func (info *FileInfo) LastSample() Sample {
	return info.lastSample
}

// Open opens the file for reading samples. The returned value
// should be closed after use. Note that the actual file is not
// opened until ReadSample is called for the second time - the
// sample already read is used to satisfy the first read.
func (info *FileInfo) Open() *SampleFile {
	return &SampleFile{
		info: info,
	}
}

// SampleFile represents an open sample file.
type SampleFile struct {
	doneFirst bool
	closed    bool
	info      *FileInfo
	r         SampleReader
	f         *os.File
}

// ReadSample implements SampleReader.ReadSample.
func (sf *SampleFile) ReadSample() (Sample, error) {
	if sf.closed {
		return Sample{}, fmt.Errorf("sample file %q: read after close", sf.info.path)
	}
	if !sf.doneFirst {
		sf.doneFirst = true
		if sf.info.firstSample.Time.IsZero() {
			return Sample{}, io.EOF
		}
		return sf.info.firstSample, nil
	}
	if sf.r == nil {
		f, err := os.Open(sf.info.path)
		if err != nil {
			return Sample{}, err
		}
		sf.f = f
		sf.r = NewSampleReader(f)
		// Read and discard the first sample that we've already returned.
		_, err = sf.r.ReadSample()
		if err != nil {
			return Sample{}, fmt.Errorf("cannot discard first sample from %q: %v", sf.info.path, err)
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
	return Sample{}, fmt.Errorf("cannot read sample from %q: %v", sf.info.path, err)
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

// readLastSample returns the last sample in the file.
func readLastSample(f *os.File) (Sample, error) {
	info, err := f.Stat()
	if err != nil {
		return Sample{}, fmt.Errorf("cannot get file info: %v", err)
	}
	// Read the last part of the file to find the final line.
	const maxLineLen = 50 // overkill - it's just an int and a float.
	if info.Size() > maxLineLen {
		_, err := f.Seek(-maxLineLen, io.SeekEnd)
		if err != nil {
			return Sample{}, fmt.Errorf("cannot seek to end of file: %v", err)
		}
	} else {
		_, err := f.Seek(0, io.SeekStart)
		if err != nil {
			return Sample{}, fmt.Errorf("cannot seek to start of file: %v", err)
		}
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return Sample{}, fmt.Errorf("cannot read last part of sample file: %v", err)
	}
	i := bytes.LastIndexByte(data, '\n')
	if i != -1 && i < len(data)-1 {
		// The file doesn't end with a newline, so it probably ends with an invalid record,
		// so ignore it.
		data = data[0 : i+1]
	}
	i = bytes.LastIndexByte(data[0:len(data)-1], '\n')
	if i >= 0 {
		data = data[i+1:]
	}
	r := NewSampleReader(bytes.NewReader(data))
	s, err := r.ReadSample()
	if err != nil {
		return Sample{}, fmt.Errorf("cannot read final sample: %v", err)
	}
	return s, nil
}

type eofReader struct{}

func (eofReader) ReadSample() (Sample, error) {
	return Sample{}, io.EOF
}

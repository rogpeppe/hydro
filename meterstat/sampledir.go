package meterstat

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

// ErrNoSamples is returned by ReadSampleDir when there are no
// sample files found.
var ErrNoSamples = fmt.Errorf("no samples found")

// ReadSampleDir reads all the files from the given directory that match the
// given glob pattern. It returns ErrNoSamples if there are no matching files
// or the directory doesn't exist.
// If pattern is empty, "*" is assumed.
func ReadSampleDir(dir string, pattern string) (*MeterSampleDir, error) {
	if pattern == "" {
		pattern = "*"
	}
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSamples
		}
		return nil, err
	}
	var files []*FileInfo
	t0 := time.Now()
	var t1 time.Time
	for _, info := range infos {
		if (info.Mode() & os.ModeType) != 0 {
			continue
		}
		match, _ := filepath.Match(pattern, info.Name())
		if !match {
			continue
		}
		path := filepath.Join(dir, info.Name())
		f, err := SampleFileInfo(path)
		if err != nil {
			continue
		}
		files = append(files, f)
		t0f, t1f := f.FirstSample().Time, f.LastSample().Time
		if t0f.Before(t0) {
			t0 = t0f
		}
		if t1f.After(t1) {
			t1 = t1f
		}
	}
	if t1.IsZero() {
		// No valid files found.
		return nil, ErrNoSamples
	}
	return &MeterSampleDir{
		Files: files,
		Range: TimeRange{t0, t1},
	}, nil
}

// MeterSampleDir represents a set of sample files in a directory.
type MeterSampleDir struct {
	// Files holds an entry for each sample file in the directory.
	Files []*FileInfo
	// Range holds the time range of samples found in the directory.
	Range TimeRange
}

// OpenRange is like Open but includes only samples from files that are needed
// to determine energy values within the specifid time range inclusive.
// If t.T0 or t.T1 are zero, d.T0 and d.T1 are used respectively.
func (d *MeterSampleDir) OpenRange(t TimeRange) SampleReadCloser {
	if t.T0.IsZero() {
		t.T0 = d.Range.T0
	}
	if t.T1.IsZero() {
		t.T1 = d.Range.T1
	}
	files := relevantFiles(d.Files, t)
	rs := make([]SampleReader, len(files))
	for i, f := range files {
		rs[i] = f.Open()
	}
	return &sampleDirReader{
		files: rs,
		r:     MultiSampleReader(rs...),
	}
}

// relevantFiles returns all of the given sample files that are useful
// for evaluating data points from t0 to t1.
// We only need to keep files that have data ranges that overlap
// the interval or that are directly before or after it if we
// don't yet have a file that overlaps [t0, t0] or [t1, t1] respectively.
func relevantFiles(sds []*FileInfo, t TimeRange) []*FileInfo {
	result := make([]*FileInfo, 0, len(sds))
	// haveStart and haveEnd record whether we've put a start or end entry into
	// result respectively.
	haveStart := false
	haveEnd := false
	var start, end *FileInfo
	for _, sd := range sds {
		sdt := sd.Range()
		if sdt.Overlaps(t) {
			result = append(result, sd)
			haveStart = haveStart || sdt.Overlaps(TimeRange{t.T0, t.T0})
			haveEnd = haveEnd || sdt.Overlaps(TimeRange{t.T1, t.T1})
			continue
		}
		if !haveStart && !sdt.T1.After(t.T0) && (start == nil || sdt.T1.After(start.Range().T1)) {
			start = sd
		}
		if !haveEnd && !sdt.T0.Before(t.T1) && (end == nil || sdt.T0.Before(end.Range().T0)) {
			end = sd
		}
	}
	if !haveStart && start != nil {
		result = append(result, start)
	}
	if !haveEnd && end != nil {
		result = append(result, end)
	}
	return result
}

// Open returns a reader that reads all the samples from the directory.
func (d *MeterSampleDir) Open() SampleReadCloser {
	return d.OpenRange(TimeRange{})
}

type sampleDirReader struct {
	files []SampleReader
	r     SampleReader
}

func (r *sampleDirReader) ReadSample() (Sample, error) {
	return r.r.ReadSample()
}

func (r *sampleDirReader) Close() error {
	for _, f := range r.files {
		f.(SampleReadCloser).Close()
	}
	return nil
}

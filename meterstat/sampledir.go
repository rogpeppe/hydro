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
		T0:    t0,
		T1:    t1,
	}, nil
}

// MeterSampleDir represents a set of sample files in a directory.
type MeterSampleDir struct {
	// Files holds an entry for each sample file in the directory.
	Files []*FileInfo
	// T0 and T1 hold the range of the sample times found in the directory.
	T0, T1 time.Time
}

// OpenRange is like Open but includes only samples from files that are needed
// to determine energy values from t0 to t1 inclusive.
// If t0 or t1 are zero, d.T0 and d.T1 are used respectively.
func (d *MeterSampleDir) OpenRange(t0, t1 time.Time) SampleReadCloser {
	if t0.IsZero() {
		t0 = d.T0
	}
	if t1.IsZero() {
		t1 = d.T1
	}
	files := relevantFiles(d.Files, t0, t1)
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
func relevantFiles(sds []*FileInfo, t0, t1 time.Time) []*FileInfo {
	result := make([]*FileInfo, 0, len(sds))
	// haveStart and haveEnd record whether we've put a start or end entry into
	// result respectively.
	haveStart := false
	haveEnd := false
	var start, end *FileInfo
	for _, sd := range sds {
		sdt0, sdt1 := sd.FirstSample().Time, sd.LastSample().Time
		if timeOverlaps(sdt0, sdt1, t0, t1) {
			result = append(result, sd)
			haveStart = haveStart || timeOverlaps(sdt0, sdt1, t0, t0)
			haveEnd = haveEnd || timeOverlaps(sdt0, sdt1, t1, t1)
			continue
		}
		if !haveStart && !sdt1.After(t0) && (start == nil || sdt1.After(start.LastSample().Time)) {
			start = sd
		}
		if !haveEnd && !sdt0.Before(t1) && (end == nil || sdt0.Before(end.FirstSample().Time)) {
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

func timeOverlaps(at0, at1, bt0, bt1 time.Time) bool {
	if at1.Before(at0) {
		panic("bad interval a")
	}
	if bt1.Before(bt0) {
		panic("bad interval b")
	}
	return !at0.After(bt1) && !bt0.After(at1)
}

// Open returns a reader that reads all the samples from the directory.
func (d *MeterSampleDir) Open() SampleReadCloser {
	return d.OpenRange(time.Time{}, time.Time{})
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

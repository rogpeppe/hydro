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
// given glob pattern. It returns ErrNoSamples if there are no matching files.
// If pattern is empty, "*" is assumed.
func ReadSampleDir(dir string, pattern string) (*MeterSampleDir, error) {
	if pattern == "" {
		pattern = "*"
	}
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
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

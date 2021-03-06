package meterstat

import (
	"bytes"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

var epoch = time.Unix(946814400, 0) // 2000-01-02 12:00:00Z

func TestSampleReader(t *testing.T) {
	c := qt.New(t)
	r := NewSampleReader(strings.NewReader(`
946814400000,1000
946814410005,1010
946814415000,23456
`[1:]))
	samples, err := ReadAllSamples(r)
	c.Assert(err, qt.IsNil)

	c.Assert(samples, qt.DeepEquals, []Sample{{
		Time:        epoch,
		TotalEnergy: 1000,
	}, {
		Time:        epoch.Add(10*time.Second + 5*time.Millisecond),
		TotalEnergy: 1010,
	}, {
		Time:        epoch.Add(15 * time.Second),
		TotalEnergy: 23456,
	}})
}

func TestWriteSamples(t *testing.T) {
	c := qt.New(t)
	data := `
946814400000,1000
946814410005,1010
946814415000,23456
`[1:]
	r := NewSampleReader(strings.NewReader(data))
	var buf bytes.Buffer
	n, err := WriteSamples(&buf, r)
	c.Assert(err, qt.IsNil)
	c.Assert(buf.String(), qt.Equals, data)
	c.Assert(n, qt.Equals, 3)
}

func TestMultiReader(t *testing.T) {
	c := qt.New(t)
	r0 := NewSampleReader(strings.NewReader(`
946814400000,1000
946814410005,1010
946814415000,2500
`[1:]))
	r1 := NewMemSampleReader([]Sample{{
		Time:        epoch.Add(30 * time.Second),
		TotalEnergy: 3000,
	}, {
		Time:        epoch.Add(36 * time.Second),
		TotalEnergy: 4000,
	}})
	r2 := NewMemSampleReader([]Sample{{
		Time:        epoch.Add(10 * time.Second),
		TotalEnergy: 1010,
	}, {
		Time:        epoch.Add(37 * time.Second),
		TotalEnergy: 4000,
	}, {
		// This entry should be discarded because the energy isn't increasing.
		Time:        epoch.Add(38 * time.Second),
		TotalEnergy: 3000,
	}})
	samples, err := ReadAllSamples(MultiSampleReader(r0, r1, r2))
	c.Assert(err, qt.IsNil)
	c.Assert(samples, qt.DeepEquals, []Sample{{
		Time:        epoch,
		TotalEnergy: 1000,
	}, {
		Time:        epoch.Add(10 * time.Second),
		TotalEnergy: 1010,
	}, {
		Time:        epoch.Add(10*time.Second + 5*time.Millisecond),
		TotalEnergy: 1010,
	}, {
		Time:        epoch.Add(15 * time.Second),
		TotalEnergy: 2500,
	}, {
		Time:        epoch.Add(30 * time.Second),
		TotalEnergy: 3000,
	}, {
		Time:        epoch.Add(36 * time.Second),
		TotalEnergy: 4000,
	}, {
		Time:        epoch.Add(37 * time.Second),
		TotalEnergy: 4000,
	}})
}

func TestSampleFile(t *testing.T) {
	c := qt.New(t)

	path := filepath.Join(t.TempDir(), "samples")
	err := ioutil.WriteFile(path, []byte(`
946814400000,1000
946814410000,1010
946814415000,1200
`[1:]), 0666)
	c.Assert(err, qt.IsNil)
	info, err := SampleFileInfo(path)
	c.Assert(err, qt.IsNil)
	sf := info.Open()
	defer func() {
		c.Check(sf.Close(), qt.IsNil)
	}()

	samples, err := ReadAllSamples(sf)
	c.Assert(err, qt.IsNil)
	c.Assert(samples, qt.DeepEquals, []Sample{{
		Time:        epoch,
		TotalEnergy: 1000,
	}, {
		Time:        epoch.Add(10 * time.Second),
		TotalEnergy: 1010,
	}, {
		Time:        epoch.Add(15 * time.Second),
		TotalEnergy: 1200,
	}})

	// Check that we continue to get io.EOF after another read.
	_, err = sf.ReadSample()
	c.Assert(err, qt.Equals, io.EOF)
}

func TestSampleFileEmpty(t *testing.T) {
	c := qt.New(t)
	path := filepath.Join(t.TempDir(), "samples")
	err := ioutil.WriteFile(path, nil, 0666)
	c.Assert(err, qt.IsNil)
	sf, err := OpenSampleFile(path)
	c.Assert(err, qt.Equals, ErrNoSamples)
	c.Assert(sf, qt.IsNil)
}

func TestSampleFileMulti(t *testing.T) {
	c := qt.New(t)

	path := filepath.Join(t.TempDir(), "samples")
	err := ioutil.WriteFile(path, []byte(`
946814400000,1000
946814410000,1010
946814415000,1200
`[1:]), 0666)
	c.Assert(err, qt.IsNil)

	// Open the same file several time. We should just get the same results anyway.
	rs := make([]SampleReader, 5)
	for i := range rs {
		sf, err := OpenSampleFile(path)
		c.Assert(err, qt.IsNil)
		defer func() {
			c.Check(sf.Close(), qt.IsNil)
		}()
		rs[i] = sf
	}
	samples, err := ReadAllSamples(MultiSampleReader(rs...))
	c.Assert(err, qt.IsNil)

	c.Assert(samples, qt.DeepEquals, []Sample{{
		Time:        epoch,
		TotalEnergy: 1000,
	}, {
		Time:        epoch.Add(10 * time.Second),
		TotalEnergy: 1010,
	}, {
		Time:        epoch.Add(15 * time.Second),
		TotalEnergy: 1200,
	}})
}

func TestSampleFileRange(t *testing.T) {
	c := qt.New(t)
	path := filepath.Join(t.TempDir(), "samples")
	err := ioutil.WriteFile(path, []byte(`
946814400000,1000
946814410000,1010
946814415000,1200
`[1:]), 0666)
	c.Assert(err, qt.IsNil)
	info, err := SampleFileInfo(path)
	c.Assert(err, qt.IsNil)
	c.Assert(info.FirstSample().Time, qt.DeepEquals, epoch)
	c.Assert(info.LastSample().Time, qt.DeepEquals, epoch.Add(15*time.Second))
}

func TestSampleFileRangeSingleSample(t *testing.T) {
	c := qt.New(t)
	path := filepath.Join(t.TempDir(), "samples")
	err := ioutil.WriteFile(path, []byte(`
946814400000,1000
`[1:]), 0666)
	c.Assert(err, qt.IsNil)
	info, err := SampleFileInfo(path)
	c.Assert(err, qt.IsNil)
	c.Assert(info.FirstSample().Time, qt.DeepEquals, epoch)
	c.Assert(info.LastSample().Time, qt.DeepEquals, epoch)
}

func TestSampleFileRangeIncompleteLastLine(t *testing.T) {
	c := qt.New(t)
	path := filepath.Join(t.TempDir(), "samples")
	err := ioutil.WriteFile(path, []byte(`
946814400000,1000
946814410000,12345
456`[1:]), 0666)
	c.Assert(err, qt.IsNil)
	info, err := SampleFileInfo(path)
	c.Assert(err, qt.IsNil)
	c.Assert(info.FirstSample().Time, qt.DeepEquals, epoch)
	c.Assert(info.LastSample().Time, qt.DeepEquals, epoch.Add(10*time.Second))
}

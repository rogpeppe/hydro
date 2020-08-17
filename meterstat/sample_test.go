package meterstat

import (
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
	samples, err := readAll(r)
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
	samples, err := readAll(MultiSampleReader(r0, r1, r2))
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
	sf, err := OpenSampleFile(path)
	c.Assert(err, qt.IsNil)
	defer func() {
		c.Check(sf.Close(), qt.IsNil)
	}()

	samples, err := readAll(sf)
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
	c.Assert(err, qt.IsNil)
	defer func() {
		c.Check(sf.Close(), qt.IsNil)
	}()
	samples, err := readAll(sf)
	c.Assert(err, qt.IsNil)
	c.Assert(samples, qt.IsNil)
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
	samples, err := readAll(MultiSampleReader(rs...))
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

func readAll(r SampleReader) ([]Sample, error) {
	var samples []Sample
	for {
		s, err := r.ReadSample()
		if err != nil {
			if err == io.EOF {
				return samples, nil
			}
			return samples, err
		}
		samples = append(samples, s)
	}
}

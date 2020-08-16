package sampleworker

import (
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/rogpeppe/hydro/ndmetertest"
)

var epoch = time.Unix(946814400, 0) // 2000-01-02 12:00:00Z

func TestWorkerSingleSample(t *testing.T) {
	c := qt.New(t)
	ndsrv, err := ndmetertest.NewServer(":0")
	c.Assert(err, qt.IsNil)
	ndsrv.SetEnergy(12300)
	timeReq := make(chan chan<- time.Time)
	p := Params{
		SampleDir: c.Mkdir(),
		MeterAddr: ndsrv.Addr,
		Prefix:    "foo-",
		Now: func() time.Time {
			tc := make(chan time.Time)
			timeReq <- tc
			return <-tc
		},
		Interval: 10 * time.Millisecond,
	}
	w, err := New(p)
	c.Assert(err, qt.IsNil)
	waitTimeReq(c, timeReq) <- epoch
	w.Close()
	select {
	case <-timeReq:
		c.Fatalf("unexpected time request after close")
	case <-time.After(20 * time.Millisecond):
	}
	assertDirContents(c, p.SampleDir, map[string]string{
		"foo-2000-01-02T120000.000Z": "946814400000,12300\n",
	})
}

func TestWorkerDayRollover(t *testing.T) {
	c := qt.New(t)
	ndsrv, err := ndmetertest.NewServer(":0")
	c.Assert(err, qt.IsNil)
	ndsrv.SetEnergy(12300)
	timeReq := make(chan chan<- time.Time)
	p := Params{
		SampleDir: c.Mkdir(),
		MeterAddr: ndsrv.Addr,
		Prefix:    "foo-",
		Now: func() time.Time {
			tc := make(chan time.Time)
			timeReq <- tc
			return <-tc
		},
		Interval: 10 * time.Millisecond,
	}
	w, err := New(p)
	c.Assert(err, qt.IsNil)
	// Wait for the first time request before setting the energy for the next one.
	tc := waitTimeReq(c, timeReq)
	ndsrv.SetEnergy(12400)
	tc <- epoch
	// Send the next timestamp before closing it.
	waitTimeReq(c, timeReq) <- epoch.Add(24 * time.Hour)
	w.Close()
	select {
	case <-timeReq:
		c.Fatalf("unexpected time request after close")
	case <-time.After(20 * time.Millisecond):
	}
	assertDirContents(c, p.SampleDir, map[string]string{
		"foo-2000-01-02T120000.000Z": "946814400000,12300\n",
		"foo-2000-01-03T120000.000Z": "946900800000,12400\n",
	})
}

func assertDirContents(c *qt.C, dir string, expectContents map[string]string) {
	infos, err := ioutil.ReadDir(dir)
	c.Assert(err, qt.IsNil)
	contents := make(map[string]string)
	for _, info := range infos {
		name := info.Name()
		data, err := ioutil.ReadFile(filepath.Join(dir, name))
		c.Assert(err, qt.IsNil)
		contents[name] = string(data)
	}
	c.Assert(contents, qt.DeepEquals, expectContents)
}

func waitTimeReq(c *qt.C, timeReq chan chan<- time.Time) chan<- time.Time {
	select {
	case tc := <-timeReq:
		return tc
	case <-time.After(time.Second):
		c.Fatalf("timed out waiting for time request")
	}
	panic("unreachable")
}

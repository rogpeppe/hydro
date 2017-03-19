package meterstore_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/rogpeppe/hydro/meterstore"
)

type suite struct{}

var _ = gc.Suite(&suite{})

var epoch = time.Date(2017, time.January, 1, 5, 30, 0, int(50*time.Millisecond), time.UTC)

func (*suite) TestSimple(c *gc.C) {
	path := filepath.Join(c.MkDir(), "db")
	store, err := meterstore.Open(path)
	c.Assert(err, gc.Equals, nil)
	defer store.Close()

	r := meterstore.TimeRecord{
		Time:          epoch,
		InLog:         true,
		MeterAddr:     "somewhere:345",
		MeterLocation: 3,
		Readings:      meterstore.SystemPower | meterstore.SystemEnergy,
		SystemPower:   1234567,
		SystemEnergy:  123456,
	}

	err = store.Add(r)
	c.Assert(err, gc.Equals, nil)

	var got []meterstore.TimeRecord
	iter := store.Iter(epoch)
	for iter.Next() {
		got = append(got, iter.Value())
	}
	c.Assert(got, jc.DeepEquals, []meterstore.TimeRecord{r})
	c.Assert(iter.Err(), gc.Equals, nil)
	store.Close()

	// Check that we can reopen the store and it still works.
	store, err = meterstore.Open(path)
	c.Assert(err, gc.Equals, nil)
	iter = store.Iter(epoch)
	got = nil
	for iter.Next() {
		got = append(got, iter.Value())
	}
	c.Check(got, jc.DeepEquals, []meterstore.TimeRecord{r})
	c.Check(iter.Err(), gc.Equals, nil)
}

var iterTestRecords = []meterstore.TimeRecord{{
	Time:          epoch,
	InLog:         false,
	MeterAddr:     "somewhere:345",
	MeterLocation: 3,
	Readings:      meterstore.SystemPower | meterstore.SystemEnergy,
	SystemPower:   1005,
	SystemEnergy:  50005,
}, {
	Time:          epoch,
	InLog:         true,
	MeterAddr:     "somewhere:345",
	MeterLocation: 3,
	Readings:      meterstore.SystemPower | meterstore.SystemEnergy,
	SystemPower:   1000,
	SystemEnergy:  50000,
}, {
	Time:          epoch.Add(2 * time.Second),
	InLog:         true,
	MeterAddr:     "somewhere:345",
	MeterLocation: 3,
	Readings:      meterstore.SystemPower,
	SystemPower:   2000,
}, {
	Time:          epoch.Add(5 * time.Second),
	InLog:         true,
	MeterAddr:     "elsewhere:345",
	MeterLocation: 2,
	Readings:      meterstore.SystemEnergy,
	SystemPower:   98765,
}, {
	Time:          epoch.Add(20 * time.Second),
	InLog:         true,
	MeterAddr:     "somewhere:345",
	MeterLocation: 3,
	Readings:      meterstore.SystemEnergy,
	SystemPower:   60000,
}}

var iterTests = []struct {
	about         string
	start         time.Time
	forward       bool
	expectIndexes []int
}{{
	about:         "forward from start",
	start:         epoch,
	forward:       true,
	expectIndexes: []int{0, 1, 2, 3, 4},
}, {
	about:         "forward from before start",
	start:         epoch.Add(-time.Minute),
	forward:       true,
	expectIndexes: []int{0, 1, 2, 3, 4},
}, {
	about:         "forward from after start",
	start:         epoch.Add(time.Second),
	forward:       true,
	expectIndexes: []int{2, 3, 4},
}, {
	about:         "forward from exactly the end",
	start:         epoch.Add(20 * time.Second),
	forward:       true,
	expectIndexes: []int{4},
}, {
	about:         "forward from beyond the end",
	forward:       true,
	start:         epoch.Add(21 * time.Second),
	expectIndexes: []int{},
}, {
	about:         "backward from end",
	start:         epoch.Add(20 * time.Second),
	expectIndexes: []int{4, 3, 2, 1, 0},
}, {
	about:         "backward from after end",
	start:         epoch.Add(21 * time.Second),
	expectIndexes: []int{4, 3, 2, 1, 0},
}, {
	about:         "backward from before end",
	start:         epoch.Add(19 * time.Second),
	expectIndexes: []int{3, 2, 1, 0},
}, {
	about:         "backward from exactly the start",
	start:         epoch,
	expectIndexes: []int{1, 0},
}, {
	about:         "backward from before the start",
	start:         epoch.Add(-time.Second),
	expectIndexes: []int{},
}}

func (*suite) TestIterForward(c *gc.C) {
	path := filepath.Join(c.MkDir(), "db")
	store, err := meterstore.Open(path)
	c.Assert(err, gc.Equals, nil)
	defer store.Close()

	err = store.Add(iterTestRecords...)
	c.Assert(err, gc.Equals, nil)
	for i, test := range iterTests {
		c.Logf("test %d: %v", i, test.about)
		var got []meterstore.TimeRecord
		var iter *meterstore.Iter
		if test.forward {
			iter = store.Iter(test.start)
		} else {
			iter = store.ReverseIter(test.start)
		}
		for iter.Next() {
			got = append(got, iter.Value())
		}
		c.Check(iter.Err(), gc.Equals, nil)
		expect := make([]meterstore.TimeRecord, len(test.expectIndexes))
		for i, index := range test.expectIndexes {
			expect[i] = iterTestRecords[index]
		}
		c.Assert(got, jc.DeepEquals, expect)
	}
}

func (*suite) TestTimestamp(c *gc.C) {
	t := time.Date(2017, time.January, 1, 5, 30, 0, int(50*time.Millisecond), time.UTC)
	stamp := meterstore.TimeToStamp(t)
	t1 := meterstore.StampToTime(stamp)
	if !t.Equal(t1) {
		c.Fatalf("got %v want %v", t, t1)
	}
}

func BenchmarkInsert(b *testing.B) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		b.Fatalf("cannot make temp file")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	store, err := meterstore.Open(f.Name())
	if err != nil {
		b.Fatalf("cannot open store")
	}
	b.ResetTimer()
	t0 := time.Now()
	for i := 0; i < b.N; i++ {
		err := store.Add([]meterstore.TimeRecord{{
			Time:          t0.Add(time.Duration(i) * 10 * time.Millisecond),
			InLog:         true,
			MeterAddr:     "hello:35",
			MeterLocation: 2,
			Readings:      meterstore.SystemPower | meterstore.SystemEnergy,
			SystemPower:   1234567,
			SystemEnergy:  123456,
		}, {
			Time:          t0.Add(time.Duration(i)*10*time.Millisecond + 5*time.Millisecond),
			InLog:         true,
			MeterAddr:     "goodbye:465",
			MeterLocation: 1,
			Readings:      meterstore.SystemPower | meterstore.SystemEnergy,
			SystemPower:   1234567,
			SystemEnergy:  123456,
		}}...)
		if err != nil {
			b.Fatalf("cannot add record %d: %v", i, err)
		}
	}
}

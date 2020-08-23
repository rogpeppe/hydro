package meterstore_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/rogpeppe/hydro/meterstore"
)

var epoch = time.Date(2017, time.January, 1, 5, 30, 0, int(50*time.Millisecond), time.UTC)

func TestSimple(t *testing.T) {
	c := qt.New(t)
	path := filepath.Join(c.Mkdir(), "db")
	store, err := meterstore.Open(path)
	c.Assert(err, qt.Equals, nil)
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
	c.Assert(err, qt.Equals, nil)

	var got []meterstore.TimeRecord
	iter := store.Iter(epoch)
	for iter.Next() {
		got = append(got, iter.Value())
	}
	c.Assert(got, qt.DeepEquals, []meterstore.TimeRecord{r})
	c.Assert(iter.Err(), qt.Equals, nil)
	store.Close()

	// Check that we can reopen the store and it still works.
	store, err = meterstore.Open(path)
	c.Assert(err, qt.Equals, nil)
	iter = store.Iter(epoch)
	got = nil
	for iter.Next() {
		got = append(got, iter.Value())
	}
	c.Check(got, qt.DeepEquals, []meterstore.TimeRecord{r})
	c.Check(iter.Err(), qt.Equals, nil)
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
	testName      string
	start         time.Time
	forward       bool
	expectIndexes []int
}{{
	testName:      "forward-from-start",
	start:         epoch,
	forward:       true,
	expectIndexes: []int{0, 1, 2, 3, 4},
}, {
	testName:      "forward-from-before-start",
	start:         epoch.Add(-time.Minute),
	forward:       true,
	expectIndexes: []int{0, 1, 2, 3, 4},
}, {
	testName:      "forward-from-after-start",
	start:         epoch.Add(time.Second),
	forward:       true,
	expectIndexes: []int{2, 3, 4},
}, {
	testName:      "forward-from-exactly-the-end",
	start:         epoch.Add(20 * time.Second),
	forward:       true,
	expectIndexes: []int{4},
}, {
	testName:      "forward-from-beyond-the-end",
	forward:       true,
	start:         epoch.Add(21 * time.Second),
	expectIndexes: []int{},
}, {
	testName:      "backward-from-end",
	start:         epoch.Add(20 * time.Second),
	expectIndexes: []int{4, 3, 2, 1, 0},
}, {
	testName:      "backward-from-after-end",
	start:         epoch.Add(21 * time.Second),
	expectIndexes: []int{4, 3, 2, 1, 0},
}, {
	testName:      "backward-from-before-end",
	start:         epoch.Add(19 * time.Second),
	expectIndexes: []int{3, 2, 1, 0},
}, {
	testName:      "backward-from-exactly-the-start",
	start:         epoch,
	expectIndexes: []int{1, 0},
}, {
	testName:      "backward-from-before-the-start",
	start:         epoch.Add(-time.Second),
	expectIndexes: []int{},
}}

func TestIterForward(t *testing.T) {
	c := qt.New(t)
	path := filepath.Join(c.Mkdir(), "db")
	store, err := meterstore.Open(path)
	c.Assert(err, qt.Equals, nil)
	defer store.Close()

	err = store.Add(iterTestRecords...)
	c.Assert(err, qt.Equals, nil)
	for _, test := range iterTests {
		c.Run(test.testName, func(c *qt.C) {
			got := []meterstore.TimeRecord{}
			var iter *meterstore.Iter
			if test.forward {
				iter = store.Iter(test.start)
			} else {
				iter = store.ReverseIter(test.start)
			}
			for iter.Next() {
				got = append(got, iter.Value())
			}
			c.Check(iter.Err(), qt.Equals, nil)
			expect := make([]meterstore.TimeRecord, len(test.expectIndexes))
			for i, index := range test.expectIndexes {
				expect[i] = iterTestRecords[index]
			}
			c.Assert(got, qt.DeepEquals, expect)
		})
	}
}

func TestTimestamp(t *testing.T) {
	c := qt.New(t)
	ts := time.Date(2017, time.January, 1, 5, 30, 0, int(50*time.Millisecond), time.UTC)
	stamp := meterstore.TimeToStamp(ts)
	ts1 := meterstore.StampToTime(stamp)
	if !ts.Equal(ts1) {
		c.Fatalf("got %v want %v", ts, ts1)
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

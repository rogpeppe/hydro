package meterstore_test

import (
	"path/filepath"
	"time"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/rogpeppe/hydro/meterstore"
)

type suite struct{}

var _ = gc.Suite(&suite{})

func (*suite) TestSimple(c *gc.C) {
	store, err := meterstore.Open(filepath.Join(c.MkDir(), "db"))
	c.Assert(err, gc.Equals, nil)

	t0 := time.Date(2017, time.January, 1, 5, 30, 0, int(50*time.Millisecond), time.UTC)
	r := meterstore.TimeRecord{
		Time:         t0,
		InLog:        true,
		Meter:        2,
		Readings:     meterstore.SystemPower | meterstore.SystemEnergy,
		SystemPower:  1234567,
		SystemEnergy: 123456,
	}

	err = store.Add(r)
	c.Assert(err, gc.Equals, nil)

	var got []meterstore.TimeRecord
	iter := store.Iter(t0)
	for iter.Next() {
		got = append(got, iter.Value())
	}
	c.Assert(got, jc.DeepEquals, []meterstore.TimeRecord{r})
	c.Assert(iter.Err(), gc.Equals, nil)
}

func (*suite) TestTimestamp(c *gc.C) {
	t := time.Date(2017, time.January, 1, 5, 30, 0, int(50*time.Millisecond), time.UTC)
	stamp := meterstore.TimeToStamp(t)
	t1 := meterstore.StampToTime(stamp)
	if !t.Equal(t1) {
		c.Fatalf("got %v want %v", t, t1)
	}
}

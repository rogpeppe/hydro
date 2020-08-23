package notifier

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestWatcher(t *testing.T) {
	c := qt.New(t)

	// blocking on the channel forces the scheduler to let the other goroutine
	// run for a bit, so we get predictable results.  This is not necessary for
	// normal use of the watcher.
	ch := make(chan bool)

	n := new(Notifier)

	var v int32
	go func() {
		for i := int32(0); i < 3; i++ {
			v = i
			n.Changed()

			ch <- true
		}
		n.Close()
	}()

	w := n.Watch()
	c.Assert(w.Next(), qt.IsTrue)
	c.Assert(v, qt.Equals, int32(0))
	<-ch

	c.Assert(w.Next(), qt.IsTrue)
	<-ch

	c.Assert(w.Next(), qt.IsTrue)
	c.Assert(v, qt.Equals, int32(2))
	<-ch

	c.Assert(w.Next(), qt.IsFalse)
}

func TestDoubleChanged(t *testing.T) {
	c := qt.New(t)

	// blocking on the channel forces the scheduler to let the other goroutine
	// run for a bit, so we get predictable results.  This is not necessary for
	// normal use of the watcher.
	ch := make(chan bool)

	n := new(Notifier)
	var v int32

	go func() {
		v = 1
		n.Changed()
		ch <- true
		v = 2
		n.Changed()
		v = 3
		n.Changed()
		ch <- true
		n.Close()
		ch <- true
	}()

	w := n.Watch()
	c.Assert(w.Next(), qt.IsTrue)
	<-ch

	// since we did two sets before sending on the channel,
	// we should just get vals[2] here and not get vals[1]
	c.Assert(w.Next(), qt.IsTrue)
	c.Assert(v, qt.Equals, int32(3))
}

func TestTwoReceivers(t *testing.T) {
	c := qt.New(t)
	var v int32

	// blocking on the channel forces the scheduler to let the other goroutine
	// run for a bit, so we get predictable results.  This is not necessary for
	// normal use of the watcher.
	ch := make(chan bool)

	n := new(Notifier)

	watcher := func() {
		w := n.Watch()
		x := 0
		for w.Next() {
			c.Check(v, qt.Equals, int32(x))
			x++
			<-ch
		}
		c.Check(x, qt.Equals, 3)
		<-ch
	}

	go watcher()
	go watcher()

	for i := 0; i < 3; i++ {
		v = int32(i)
		n.Changed()
		ch <- true
		ch <- true
	}

	n.Close()
	ch <- true
	ch <- true
}

func TestCloseWatcher(t *testing.T) {
	c := qt.New(t)

	// blocking on the channel forces the scheduler to let the other goroutine
	// run for a bit, so we get predictable results.  This is not necessary for
	// normal use of the watcher.
	ch := make(chan bool)

	n := new(Notifier)
	v := int32(0)
	w := n.Watch()
	go func() {
		x := 0
		for w.Next() {
			c.Check(v, qt.Equals, int32(x)+1)
			x++
			<-ch
		}
		// the value will only get set once before the watcher is closed
		c.Check(x, qt.Equals, 1)
		<-ch
	}()

	v = 1
	n.Changed()
	ch <- true
	w.Close()
	ch <- true

	// prove the value is not closed, even though the watcher is
	c.Assert(n.Closed(), qt.IsFalse)
}

func TestWatchZeroValue(t *testing.T) {
	c := qt.New(t)
	var n Notifier
	ch := make(chan bool)
	go func() {
		w := n.Watch()
		ch <- true
		ch <- w.Next()
	}()
	<-ch
	n.Changed()
	c.Assert(<-ch, qt.IsTrue)
}

// Package ntpclock provides an NTP-backed source of time values
// for use when the system clock can't always be relied upon to produce
// NTP-synchronized timestamps.
//
// TODO add tests.
package ntpclock

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/beevik/ntp"
)

const DefaultNTPHost = "pool.ntp.org"

type Clock struct {
	closed   chan struct{}
	location *time.Location
	// host holds the NTP host to use
	host string
	// mu guards the fields below it.
	mu sync.Mutex
	// t0 holds the system clock time
	t0 time.Time
	// absT0 holds the absolute time corresponding to t0.
	absT0 time.Time
	// prevTime holds the previous time reading returned from Now.
	prevTime time.Time
}

// ntpQuery is used to query the current NTP time.
// It's overridden for tests.
var ntpQuery = ntp.QueryWithOptions

const DefaultHost = "pool.ntp.org"
const DefaultTimeout = 30 * time.Second

type Params struct {
	// Host holds the the NTP host to use.
	Host string
	// Timeout holds the timeout on making the initial Clock instance.
	Timeout time.Duration
	// Location holds the time zone location to use for the returned time.
	// If it's empty, UTC is used.
	Location string
}

// New returns a Clock that queries the given NTP host for time.
// New might block for up to the given timeout while it tries to
// find out the time, or DefaultTimeout if timeout is zero.
// The Clock should be closed after use.
// If host is empty, DefaultHost will be used.
func New(p Params) (*Clock, error) {
	if p.Host == "" {
		p.Host = DefaultNTPHost
	}
	if p.Timeout == 0 {
		p.Timeout = DefaultTimeout
	}
	c := &Clock{
		host:   p.Host,
		closed: make(chan struct{}),
	}
	if p.Location != "" {
		loc, err := time.LoadLocation(p.Location)
		if err != nil {
			return nil, fmt.Errorf("cannot load timezone %q: %v", p.Location, err)
		}
		c.location = loc
	}
	err := c.update(p.Timeout)
	if err != nil {
		return nil, err
	}
	go c.updater()
	return c, nil
}

// Now returns a best-effort representation of the absolute time.
// The returned time does not contain a monotonic clock reading..
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.absT0.Add(time.Since(c.t0))
	// Try to make sure that the time increases monotonically.
	// This can't work across restarts, of course.
	if t.Before(c.prevTime) {
		return c.prevTime
	}
	if c.location != nil {
		t = t.In(c.location)
	}
	c.prevTime = t
	return t
}

func (c *Clock) updater() {
	for {
		select {
		case <-c.closed:
			return
		case <-time.After(30 * time.Minute):
		}
		if err := c.update(20 * time.Second); err != nil {
			log.Printf("cannot update time from NTP: %v", err)
		}
	}
}

func (c *Clock) update(timeout time.Duration) error {
	resp, err := ntpQuery(c.host, ntp.QueryOptions{
		Timeout: timeout,
	})
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t0 = time.Now()
	c.absT0 = c.t0.Add(resp.ClockOffset).Round(0)
	return nil
}

func (c *Clock) Close() {
	close(c.closed)
}

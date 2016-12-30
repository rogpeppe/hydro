package hydroctl

/*

TODO
max power - estimate
when max power is known, do not turn on discretionary-power
relays such that the possible total max power is greater
than the available total power.

when we don't know max power for a relay with confidence,
make sure that we switch that relay on only when no other
relays are switched. then we have a reasonable chance of
measuring the power difference before and after the switch.
We can keep a log of those differences for a relay and
estimate the max power by using a statistical measure
(e.g. 90% quantile). [would a histogram estimation be better?]
*/

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gopkg.in/errgo.v1"
)

var Debug = true

// MinimumChangeDuration holds the minimum length of time
// between turning on relays.
const MinimumChangeDuration = 5 * time.Second

// Config holds the configuration of the control system.
type Config struct {
	// Relays holds the configuration for all the relays
	// in the system. The relay number is indicated by
	// the index in the slice.
	Relays []RelayConfig
}

// RelayConfig holds the configuration for a given relay.
// The zero value is always off.
type RelayConfig struct {
	Mode RelayMode

	// MaxPower holds the maximum power that the given relay
	// can draw, in watts.
	MaxPower int

	InUse    []*Slot
	NotInUse []*Slot

	// Cohort holds the cohort that this relay is a part
	// of. This is for informational purposes only.
	Cohort string
}

// At returns the slot that is applicable to the given time
// and the absolute time of the start of the slot.
// If there is no slot for the given time, it returns nil.
func (c *RelayConfig) At(t time.Time) (slot *Slot, start time.Time) {
	var slots []*Slot
	switch c.Mode {
	case AlwaysOff, AlwaysOn:
		return nil, time.Time{}
	case InUse:
		slots = c.InUse
	case NotInUse:
		slots = c.NotInUse
	default:
		panic("unexpected mode")
	}
	for _, slot := range slots {
		if start := slot.ActiveAt(t); !start.IsZero() {
			return slot, start
		}
	}
	return nil, time.Time{}
}

type RelayMode int

const (
	AlwaysOff RelayMode = iota
	AlwaysOn
	InUse
	NotInUse
)

//go:generate stringer -type SlotKind

type SlotKind int

const (
	_ SlotKind = iota
	AtLeast
	AtMost
	Exactly
)

// Slot holds the configuration for a given time slot in a relay.
// For example, a storage radiator that needs to be on for
// at least 3 hours in the night might be specified with:
//
//	Rule{
//		Start: 23 * time.Hour,,
//		Duration: 6 * time.Hour,
//		Kind: AtLeast,
//		Duration: 3 * time.Hour
//	}
type Slot struct {
	// Start holds the slot start time, measured as a duration
	// from midnight. It must be less than 24 hours.
	Start time.Duration

	// SlotDuration holds the duration of the slot. It must be greater
	// than zero and less than or equal to 24 hours.
	SlotDuration time.Duration

	// Kind holds the kind of slot this is.
	Kind SlotKind

	// Duration holds the duration for the kind.
	Duration time.Duration
}

func (slot *Slot) String() string {
	return fmt.Sprintf("[slot %s %v; %v for %v]", slot.Start, slot.SlotDuration, slot.Kind, slot.Duration)
}

// ActiveAt returns whether the slot is active at the
// given time. If so, it returns the time the slot started;
// if not it returns the zero time.
func (slot *Slot) ActiveAt(t time.Time) time.Time {
	start := dayStart(t).Add(slot.Start)
	end := start.Add(slot.SlotDuration)
	if !t.Before(start) && t.Before(end) {
		return start
	}
	return time.Time{}
}

// Overlaps reports whether the two slots overlap in time.
// If a slot has zero duration, it is not considered to overlap
// any other slot.
func (slot0 *Slot) Overlaps(slot1 *Slot) bool {
	if slot0.SlotDuration == 0 || slot1.SlotDuration == 0 {
		return false
	}
	slot0end := slot0.Start + slot0.SlotDuration
	slot1end := slot1.Start + slot1.SlotDuration
	return slot0.Start < slot1end && slot1.Start < slot0end
}

// dayStart returns the start of the day containing the given time.
// TODO what about time zone changes?
func dayStart(t time.Time) time.Time {
	dayOffset :=
		time.Duration(t.Hour())*time.Hour +
			time.Duration(t.Minute())*time.Minute +
			time.Duration(t.Second())*time.Second +
			time.Duration(t.Nanosecond())
	return t.Add(-dayOffset)
}

func (cfg *Config) SetSlot(relay int, slot int, rule Slot) error {
	panic("not implemented")
}

func (cfg *Config) Commit() error {
	panic("not implemented")
}

// History provides access to the relay state history.
type History interface {
	// OnDuration returns the length of time that the given
	// relay has been switched on within the given time interval.
	OnDuration(relay int, t0, t1 time.Time) time.Duration

	// LatestChange returns current state of the given relay and
	// the time at which it changed to that state.
	// If there is no previous change, it returns (false, time.Time{}).
	LatestChange(relay int) (bool, time.Time)
}

// MeterReading holds the current meter readings.
type MeterReading struct {
	// Import holds the current amount of power being
	// imported in watts. If electricity is currently being
	// exported, this will be negative.
	Import int

	// Here holds the amount of power currently being
	// used by the house we're controlling in watts.
	Here int

	// Neighbour holds the amount of power currently being
	// used by the neighbour's house in watts.
	Neighbour int
}

// Total returns the total amount of power available in watts.
func (m MeterReading) Total() int {
	return (m.Here + m.Neighbour) - m.Import
}

type priority int

const (
	priLow priority = iota
	priHigh
	priAbsolute
)

//go:generate stringer -type priority

// MaxRelayCount holds the maximum number of relays
// the system can be configured with.
const MaxRelayCount = 32

// RelayState holds the state of a set of relays.
type RelayState uint32

// IsSet reports whether the given relay is on.
func (s RelayState) IsSet(relay int) bool {
	if relay < 0 || relay >= MaxRelayCount {
		panic(errgo.Newf("relay %d out of bounds", relay))
	}
	return (s & (1 << uint(relay))) != 0
}

// Set sets the given relay to the given state.
func (s *RelayState) Set(relay int, on bool) {
	if relay < 0 || relay >= MaxRelayCount {
		panic(errgo.Newf("relay %d out of bounds", relay))
	}
	if on {
		*s |= 1 << uint(relay)
	} else {
		*s &^= 1 << uint(relay)
	}
}

// String returns a string representation of the relay
// state in the form [0 2 5]
func (s RelayState) String() string {
	var parts []string
	for i := 0; s != 0; i++ {
		if s&1 != 0 {
			parts = append(parts, fmt.Sprint(i))
		}
		s >>= 1
	}
	return "[" + strings.Join(parts, " ") + "]"
}

type assessedRelay struct {
	relay      int
	pri        priority
	onDuration time.Duration
}

// canSetRelay reports whether the given relay may
// be switched to the given on value. We don't change
// the state of any relay if it's been in the other state
// recently.
func (a assessor) canSetRelay(hist History, relay int, on bool, now time.Time) bool {
	latestOn, t := hist.LatestChange(relay)
	if on == latestOn {
		return true
	}
	if t.IsZero() || now.Sub(t) >= MinimumChangeDuration {
		return true
	}
	a.logf("too soon to set relay %v (latestOn %v; delta %v)", relay, latestOn, now.Sub(t))
	return false
}

// Logger is the interface used by Assess to log the reasons for the assessment.
type Logger interface {
	Log(s string)
}

type assessor struct {
	logger Logger
}

func (a assessor) logf(f string, args ...interface{}) {
	if a.logger != nil {
		a.logger.Log(fmt.Sprintf(f, args...))
	}
}

// Assess assesses what the new state of the power-controlling relays should be
// by looking at the given history, configuration and current meter reading.
//
// It ensures that no more than one relay is turned on within MinimumChangeDuration
// to prevent power surges, and similarly that if a relay was turned on or off recently, we
// don't change its state too soon.
func Assess(cfg *Config, currentState RelayState, hist History, meter MeterReading, logger Logger, now time.Time) RelayState {
	a := assessor{
		logger: logger,
	}
	newState := currentState
	assessed := make([]assessedRelay, 0, len(cfg.Relays))

	// Find the earliest start time of any of the current slots,
	// so that we can order relays by the amount of time
	// they've been switched on since then. Limit searching
	// by starting at most 24 hours ago, and only include
	// relays in discretionary power mode.
	earliestStart := now
	earliestPossibleStart := now.Add(-24 * time.Hour)
	added := -1 // Number of first relay with absolute priority to be turned on.
	for i, rc := range cfg.Relays {
		on, pri := a.assessRelay(i, &rc, hist, now)
		if pri == priAbsolute {
			a.logf("relay %d has absolute priority %v (current state %v)", i, pri, currentState.IsSet(i))
			if on {
				if !currentState.IsSet(i) && added == -1 {
					// The relay is not already on and we haven't found
					// any other relay being turned on.
					added = i
				}
			} else if a.canSetRelay(hist, i, false, now) {
				newState.Set(i, false)
			}
			continue
		}
		slot, start := rc.At(now)
		if slot == nil {
			panic("discretionary relay without a time slot!")
		}
		if start.Before(earliestPossibleStart) {
			start = earliestPossibleStart
		}
		if start.Before(earliestStart) {
			earliestStart = start
		}
		assessed = append(assessed, assessedRelay{
			relay: i,
			pri:   pri,
		})
	}

	latestOnTime := allRelaysLatestOnTime(hist, len(cfg.Relays))

	// canTurnOn holds whether we're allowed to turn on any
	// relay because the last time we turned on any relay
	// was long enough ago. We always allow turning relays
	// off, but we turn them on slowly.
	canTurnOn := !now.Before(latestOnTime.Add(MinimumChangeDuration))

	if added != -1 && canTurnOn {
		// Absolute priority requirements have resulted in
		// a relay turning on. Turn all discretionary
		// power off until we can assess the results of this new
		// change.
		// TODO only turn off discretionary power if there's
		// not enough power available to cope with the
		// max power usable by the newly added relay.
		for _, ar := range assessed {
			if a.canSetRelay(hist, ar.relay, false, now) {
				newState.Set(ar.relay, false)
			}
		}
		newState.Set(added, true)
		return newState
	}
	for i := range assessed {
		assessed[i].onDuration = hist.OnDuration(i, earliestStart, now)
	}
	sort.Sort(assessedByPriority(assessed))
	for i, ar := range assessed {
		a.logf("sort %d: relay %d; pri %v; on %v", i, ar.relay, ar.pri, ar.onDuration)
	}
	a.logf("meter import %v", meter.Import)
	if meter.Import > 0 {
		// We're importing electricity. This must stop forthwith.
		// How do we decide how many meters to turn off?
		// If we turn off all discretionary relays then we can get
		// into a nasty cycle:
		//	- turn on relay 1
		//	- turn on relay 2
		// 	- turn on relay 3
		//	- oops, we're importing
		//	- turn off relay 1, 2, 3
		//	- turn on relay 1
		//	- turn on relay 2
		// 	- turn on relay 3
		//	- oops, we're importing
		//	- turn off relay 1, 2, 3
		//	- etc
		// So we really want to make one adjustment at a time,
		// but that might not be good enough.
		// So we switch off just enough relays that we hope we'll stop importing.
		// TODO better algorithm for deciding which order to choose relays
		// to switch off.

		// regain is the measure of how much power we need
		// to stop using to get under the required limit.
		regain := meter.Here - meter.Total()/2
		a.logf("need to regain %d", regain)
		for _, r := range assessed {
			if regain <= 0 {
				break
			}
			if !currentState.IsSet(r.relay) {
				// Relay is already off - we won't change anything if we switch it off.
				continue
			}
			if !a.canSetRelay(hist, r.relay, false, now) {
				a.logf("would like to turn off %d but can't", r.relay)
				continue
			}
			a.logf("regaining by turning off %v", r.relay)
			newState.Set(r.relay, false)
			regain -= cfg.Relays[r.relay].MaxPower
		}
	} else if canTurnOn {
		a.logf("we can turn on something")
		for i := len(assessed) - 1; i >= 0; i-- {
			ar := &assessed[i]
			if currentState.IsSet(ar.relay) {
				// The relay is already on; leave it that way.
				a.logf("%d is already on; leaving it that way", ar.relay)
				continue
			}
			if a.canSetRelay(hist, ar.relay, true, now) {
				// Turn on just the one relay.
				a.logf("turning on %d", ar.relay)
				newState.Set(ar.relay, true)
				break
			}
			a.logf("would like to turn on %d but can't", ar.relay)
		}
	}
	return newState
}

// allRelaysLatestOnTime returns the latest time that any of
// the relays in [0, n) was switched on. If none
// of them are on, it returns the zero time.
// TODO investigate the possibility that this could
// be more efficiently implemented if defined on
// History interface.
func allRelaysLatestOnTime(h History, n int) time.Time {
	var t time.Time
	for i := 0; i < n; i++ {
		on, ont := h.LatestChange(i)
		if on && ont.After(t) {
			t = ont
		}
	}
	return t
}

// assessedByPriority defines an ordering for relays
// that can use discretionary power; when sorted,
// higher indexes have higher priority.
type assessedByPriority []assessedRelay

func (ap assessedByPriority) Less(i, j int) bool {
	a0, a1 := ap[i], ap[j]
	if a0.pri != a1.pri {
		// Higher priority wins.
		return a0.pri < a1.pri
	}
	if a0.onDuration != a1.onDuration {
		// Less time on wins
		return a0.onDuration > a1.onDuration
	}
	// Break ties with relay number - lower relay numbers
	// have higher priority.
	return a0.relay > a1.relay
}

func (ap assessedByPriority) Swap(i, j int) {
	ap[i], ap[j] = ap[j], ap[i]
}

func (ap assessedByPriority) Len() int {
	return len(ap)
}

func (a assessor) assessRelay(relay int, rc *RelayConfig, hist History, now time.Time) (on bool, pri priority) {
	on, pri = a.assessRelay0(relay, rc, hist, now)
	a.logf("assessRelay %d -> %v %v", relay, on, pri)
	return
}

// assessRelay assesses the desired status of the given relay with
// respect to its configuration and history at the given time.
// It returns the desired state and how important it is to
// put the relay in that state.
func (a assessor) assessRelay0(relay int, rc *RelayConfig, hist History, now time.Time) (on bool, pri priority) {
	switch rc.Mode {
	case AlwaysOff:
		a.logf("always off")
		return false, priAbsolute
	case AlwaysOn:
		a.logf("always on")
		return true, priAbsolute
	}
	slot, start := rc.At(now)
	if slot == nil {
		a.logf("no slot at %v", now)
		return false, priAbsolute
	}
	dur := hist.OnDuration(relay, start, now)
	a.logf("got slot %v starting at %v, has %v", slot, D(start), dur)

	switch {
	case (slot.Kind == Exactly || slot.Kind == AtLeast) && start.Add(slot.SlotDuration).Sub(now) <= slot.Duration-dur:
		a.logf("must use all remaining time")
		// All the remaining time must be used.
		return true, priAbsolute
	case (slot.Kind == Exactly || slot.Kind == AtMost) && dur >= slot.Duration:
		a.logf("already had the time")
		// Already had the time we require.
		return false, priAbsolute
	case slot.Kind == Exactly || slot.Kind == AtLeast:
		a.logf("want more discretionary time")
		return true, priHigh
	case slot.Kind == AtMost:
		a.logf("could use more time")
		return true, priLow
	default:
		panic("unreachable")
	}
}

// TODO delete me
var epoch = time.Date(2000, 01, 01, 0, 0, 0, 0, time.UTC)

// TODO delete me
func D(t time.Time) time.Duration {
	return t.Sub(epoch)
}

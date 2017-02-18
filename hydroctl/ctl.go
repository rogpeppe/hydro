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

// CycleDuration holds the length of time that we will cycle
// through relays using discretionary power.
const CycleDuration = 5 * time.Minute

// MeterReactionDuration holds the length of time we wait
// for the meters to react to relay changes before
// we make further decisions.
const MeterReactionDuration = 10 * time.Second

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
	// TODO redefine as float64 for consistency.
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

// canSet reports whether the assessed relay may
// be switched to the given on value. We don't change
// the state of any relay if it's been in the other state
// recently.
func (a *assessor) canSetRelay(r *assessedRelay, on bool, now time.Time) bool {
	if on == r.latestState {
		return true
	}
	if r.latestStateDuration >= MinimumChangeDuration {
		return true
	}
	a.logf("too soon to set relay %v (latestState %v; delta %v)", r.relay, r.latestState, r.latestStateDuration)
	return false
}

// Logger is the interface used by Assess to log the reasons for the assessment.
type Logger interface {
	Log(s string)
}

type assessor struct {
	AssessParams
}

func (a *assessor) logf(f string, args ...interface{}) {
	if a.Logger != nil {
		a.Logger.Log(fmt.Sprintf(f, args...))
	}
}

// AssessParams holds parameters used in assessing
// a hydro control decision.
type AssessParams struct {
	Config         *Config
	CurrentState   RelayState
	History        History
	PowerUseSample PowerUseSample
	Logger         Logger
	Now            time.Time
}

// PowerUseSample holds a power use calculation that uses
// meter readings gathered over a period of time.
type PowerUseSample struct {
	PowerUse
	// T0 and T1 hold the range of times from which
	// the data has been gathered.
	T0, T1 time.Time
}

// Assess assesses what the new state of the power-controlling relays should be
// by looking at the given history, configuration and current meter reading.
//
// It ensures that no more than one relay is turned on within MinimumChangeDuration
// to prevent power surges, and similarly that if a relay was turned on or off recently, we
// don't change its state too soon.
func Assess(p AssessParams) RelayState {
	a := &assessor{
		AssessParams: p,
	}
	newState := a.CurrentState
	// assessed will hold all the relays that want discretionary power.
	assessed := make([]assessedRelay, 0, len(a.Config.Relays))

	// Find the earliest start time of any of the current slots,
	// so that we can order relays by the amount of time
	// they've been switched on since then. Limit searching
	// by starting at most 24 hours ago, and only include
	// relays in discretionary power mode.
	// Also, make any changes to relays with absolute priority.
	earliestStart := a.Now
	earliestPossibleStart := a.Now.Add(-24 * time.Hour)
	added := -1 // Number of first relay with absolute priority to be turned on.
	for i, rc := range a.Config.Relays {
		ar := a.assessRelay(i, &rc)
		if ar.pri == priAbsolute {
			a.logf("relay %d has absolute priority %v (current state %v)", i, ar.pri, a.CurrentState.IsSet(i))
			if ar.desiredState {
				if !a.CurrentState.IsSet(i) && added == -1 {
					// The relay is not already on and we haven't found
					// any other relay being turned on.
					added = i
				}
			} else if a.canSetRelay(&ar, false, a.Now) {
				newState.Set(i, false)
			}
			continue
		}
		slot, start := rc.At(a.Now)
		if slot == nil {
			panic("discretionary relay without a time slot!")
		}
		if start.Before(earliestPossibleStart) {
			start = earliestPossibleStart
		}
		if start.Before(earliestStart) {
			earliestStart = start
		}
		assessed = append(assessed, ar)
	}

	latestChangeTime, latestOnTime := allRelaysLatestChange(a.History, len(a.Config.Relays))

	// canTurnOn holds whether we're allowed to turn on any
	// relay because the last time we turned on any relay
	// was long enough ago. We always allow turning relays
	// off, but we turn them on slowly.
	canTurnOn := !a.Now.Before(latestOnTime.Add(MinimumChangeDuration))

	if added != -1 && canTurnOn {
		// Absolute priority requirements have resulted in
		// a relay turning on. Turn all discretionary
		// power off until we can assess the results of this new
		// change.
		// TODO only turn off discretionary power if there's
		// not enough power available to cope with the
		// max power usable by the newly added relay.
		for _, ar := range assessed {
			if a.canSetRelay(&ar, false, a.Now) {
				newState.Set(ar.relay, false)
			}
		}
		newState.Set(added, true)
		return newState
	}

	// From here on, we'll be using the meter readings to determine
	// our action. If the meter readings aren't up to date, then don't
	// do anything more, because we don't want to make decisions
	// based on data that doesn't correspond to the current relay state.
	a.logf("meter readings at %v; latest change time %v", a.PowerUseSample.T0, latestChangeTime)

	if a.PowerUseSample.T0.IsZero() {
		a.logf("invalid meter time (zero time)")
		return newState
	}

	if a.PowerUseSample.T0.Before(latestChangeTime) {
		a.logf("meter readings out of date, leaving discretionary power unchanged; reading at %s, not after %s", a.PowerUseSample.T0, latestChangeTime)
		return newState
	}
	settledTime := latestChangeTime.Add(MeterReactionDuration)
	if a.PowerUseSample.T0.Before(settledTime) {
		a.logf("meter readings not settled yet (settled in %v, reading %v ago)", settledTime.Sub(a.Now), a.Now.Sub(a.PowerUseSample.T0))
		return newState
	}
	for i := range assessed {
		assessed[i].onDuration = a.History.OnDuration(i, earliestStart, a.Now)
	}
	sort.Sort(assessedByPriority(assessed))
	for i, ar := range assessed {
		a.logf("sort %d: relay %d; pri %v; on %v", i, ar.relay, ar.pri, ar.onDuration)
	}
	pc := ChargeablePower(a.PowerUseSample.PowerUse)
	a.logf("meter import %v", pc.ImportHere)
	if pc.ImportHere > 0 {
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
		a.regainPower(&newState, assessed, pc.ImportHere, false)
		return newState
	}
	if !canTurnOn {
		return newState
	}
	a.logf("we may be able to turn on something")
	// Traverse from high to low priority.
	alreadyOn := false
	for i := len(assessed) - 1; i >= 0; i-- {
		ar := &assessed[i]
		if a.CurrentState.IsSet(ar.relay) {
			// The relay is already on; leave it that way.
			a.logf("%d is already on; leaving it that way", ar.relay)
			alreadyOn = true
			continue
		}
		if imp := a.possibleImport(ar.relay); imp > 0 {
			if !alreadyOn && a.regainPower(&newState, assessed, imp, true) {
				// There's no higher priority relay that's already on and
				// we've turned off some relays, so hopefully we that will
				// give us enough power back that the next time we
				// assess the situation we'll be able to turn on the
				// current relay.
				a.logf("regained power in order to turn on %d", ar.relay)
				break
			}
			a.logf("would like to turn on %d but not enough available power", ar.relay)
			continue
		}
		if a.canSetRelay(ar, true, a.Now) {
			// Turn on just the one relay.
			a.logf("turning on %d", ar.relay)
			newState.Set(ar.relay, true)
			break
		}
		a.logf("would like to turn on %d but can't", ar.relay)
	}
	return newState
}

// regainPower tries to turn off enough relays to regain the given
// amount of power. If must is true, no change will be made if it's
// not possible to regain all the required power.
// It reports whether the goal was achieved.
func (a *assessor) regainPower(state *RelayState, assessed []assessedRelay, regain float64, must bool) bool {
	newState := *state
	a.logf("trying to regain %v", regain)
	// Note: we traverse from least priority to highest priority.
	for _, ar := range assessed {
		if regain <= 0 {
			break
		}
		if !a.CurrentState.IsSet(ar.relay) {
			// Relay is already off - we won't change anything if we switch it off.
			continue
		}
		if !a.canSetRelay(&ar, false, a.Now) {
			a.logf("would like to turn off %d but can't", ar.relay)
			continue
		}
		a.logf("regaining by turning off %v", ar.relay)
		newState.Set(ar.relay, false)
		regain -= float64(a.Config.Relays[ar.relay].MaxPower)
	}
	if regain <= 0 || !must {
		*state = newState
		return true
	}
	return false
}

// allRelaysLatestOnTime returns the latest time
// that any of the relays in [0, n) was changed
// and the latest time that any of them was switched on.
// If none of them have changed, anyTime will hold the
// zero time; if none of them are on it onTime will hold
// the zero time.
// TODO investigate the possibility that this could
// be more efficiently implemented if defined on
// History interface.
func allRelaysLatestChange(h History, n int) (anyTime, onTime time.Time) {
	for i := 0; i < n; i++ {
		on, t := h.LatestChange(i)
		if on && t.After(onTime) {
			onTime = t
		}
		if t.After(anyTime) {
			anyTime = t
		}
	}
	return anyTime, onTime
}

// assessedRelay holds information about a relay that's being assessed.
type assessedRelay struct {
	// relay holds the relay number.
	relay int

	// desiredState holds the state that the relay wants
	// to be in.
	desiredState bool

	// pri holds the priority of that state.
	pri priority

	// onDuration holds the total amount of time
	// the relay has been on since the slot start
	// of any of the assessed relays. This field
	// is not set by assessRelay.
	onDuration time.Duration

	// latestState holds the latest known state of the
	// relay.
	latestState bool

	// latestStateDuration holds the duration that the
	// relay has been in its latest state, up to a maximum
	// of 24 hours.
	latestStateDuration time.Duration
}

// assessedByPriority defines an ordering for relays
// that can use discretionary power; when sorted,
// higher indexes have higher priority.
type assessedByPriority []assessedRelay

// Less implements sort.Interface.Less by reporting whether
// ap[i] has less priority than ap[j].
func (ap assessedByPriority) Less(i, j int) bool {
	a0, a1 := ap[i], ap[j]
	if a0.pri != a1.pri {
		// Higher priority wins.
		return a0.pri < a1.pri
	}
	// If both relays want to be on only one of them currently is,
	// and the one that's on has been on for less than the
	// cycle time, then we want to leave that one on.
	if a0.desiredState && a1.desiredState {
		// Both relays want to be on.
		// inCycle[01] holds whether a[01] is currently inside a cycle time.
		inCycle0 := a0.latestState && a0.latestStateDuration < CycleDuration
		inCycle1 := a1.latestState && a1.latestStateDuration < CycleDuration
		if inCycle0 != inCycle1 {
			// Only one of them is within its cycle time - the one
			// that isn't gets less priority so that we will prefer
			// to leave a relay on within its cycle.
			return !inCycle0
		}
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

// possibleImport reports the amount of import power that turning
// on the given relay might use.
func (a *assessor) possibleImport(relay int) float64 {
	pu := a.PowerUseSample.PowerUse
	pu.Here += float64(a.Config.Relays[relay].MaxPower)
	return ChargeablePower(pu).ImportHere
}

// assessRelay assesses the desired status of the given relay with
// respect to its configuration and history at the given time. It
// returns a summary of the relay's assessed state.
func (a *assessor) assessRelay(relay int, rc *RelayConfig) assessedRelay {
	on, pri := a.assessRelay0(relay, rc)
	latestState, latestChangeTime := a.History.LatestChange(relay)
	ar := assessedRelay{
		relay:               relay,
		desiredState:        on,
		pri:                 pri,
		latestState:         latestState,
		latestStateDuration: 24 * time.Hour,
	}
	if !latestChangeTime.IsZero() {
		if d := a.Now.Sub(latestChangeTime); d < 24*time.Hour {
			ar.latestStateDuration = d
		}
	}
	a.logf("assessRelay %d -> %v %v", relay, on, pri)
	return ar
}

// assessRelay assesses the desired status of the given relay with
// respect to its configuration and history at the given time. It
// returns the desired state and how important it is to put the relay in
// that state.
func (a *assessor) assessRelay0(relay int, rc *RelayConfig) (on bool, pri priority) {
	switch rc.Mode {
	case AlwaysOff:
		a.logf("always off")
		return false, priAbsolute
	case AlwaysOn:
		a.logf("always on")
		return true, priAbsolute
	}
	slot, start := rc.At(a.Now)
	if slot == nil {
		a.logf("no slot at %v", a.Now)
		return false, priAbsolute
	}
	dur := a.History.OnDuration(relay, start, a.Now)
	a.logf("got slot %v starting at %v, has %v", slot, D(start), dur)

	switch {
	case (slot.Kind == Exactly || slot.Kind == AtLeast) && start.Add(slot.SlotDuration).Sub(a.Now) <= slot.Duration-dur:
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

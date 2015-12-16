package hydroserver

import (
	"strconv"
	"strings"
	"time"

	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/hydroctl"
)

// TODO return errors that point to the field that's in error.

func parseState(st *State) (*hydroctl.Config, error) {
	maxRelay := -1
	type relayInfo struct {
		cohort *Cohort
		config hydroctl.RelayConfig
	}
	relays := make(map[int]relayInfo)
	for _, c := range st.Cohorts {
		for _, r := range c.Relays {
			if r >= hydroctl.MaxRelayCount {
				return nil, errgo.Newf("cohort has out-of-bound relay number %d", r)
			}
			if r > maxRelay {
				maxRelay = r
			}
			if oldc := relays[r]; oldc.cohort != nil {
				return nil, errgo.Newf("cohort %q uses duplicate relay %v (already in use by %q)", c.Id, r, oldc.cohort.Id)
			}
			cfg, err := parseCohort(c)
			if err != nil {
				return nil, errgo.Notef(err, "cohort %q", c.Id)
			}
			relays[r] = relayInfo{
				cohort: c,
				config: cfg,
			}
		}
	}
	cfg := &hydroctl.Config{
		Relays: make([]hydroctl.RelayConfig, maxRelay+1),
	}
	for r, c := range relays {
		cfg.Relays[r] = c.config
	}
	return cfg, nil
}

func parseCohort(c *Cohort) (hydroctl.RelayConfig, error) {
	var cfg hydroctl.RelayConfig
	var ok bool
	cfg.Mode, ok = modes[c.Mode]
	if !ok {
		return hydroctl.RelayConfig{}, errgo.Newf("unknown mode %q", c.Mode)
	}
	var err error
	cfg.MaxPower, err = parsePower(c.MaxPower)
	if err != nil {
		return hydroctl.RelayConfig{}, errgo.Mask(err)
	}
	cfg.InUse, err = parseSlots(c.InUseSlots)
	if err != nil {
		return hydroctl.RelayConfig{}, errgo.Mask(err)
	}
	cfg.NotInUse, err = parseSlots(c.InUseSlots)
	if err != nil {
		return hydroctl.RelayConfig{}, errgo.Mask(err)
	}
	return cfg, nil
}

func parseSlots(slots []Slot) ([]*hydroctl.Slot, error) {
	ctlSlots := make([]*hydroctl.Slot, len(slots))
	for i, slot := range slots {
		ctlSlot, err := parseSlot(slot)
		if err != nil {
			return nil, errgo.Mask(err)
		}
		for j, ctlSlot1 := range ctlSlots[:i] {
			if slotOverlap(ctlSlot, ctlSlot1) {
				return nil, errgo.Newf("overlapping slot %d vs %d", i, j)
			}
		}
		ctlSlots[i] = ctlSlot
	}
	return ctlSlots, nil
}

var slotsKinds = map[string]hydroctl.SlotKind{
	">=": hydroctl.AtLeast,
	"<=": hydroctl.AtMost,
	"==": hydroctl.Exactly,
}

func parseSlot(slot Slot) (*hydroctl.Slot, error) {
	var ctlSlot hydroctl.Slot
	var ok bool
	// TODO if slot.Kind is empty and slot.Duration is empty
	// then we could defined to "==" and SlotDuration.
	ctlSlot.Kind, ok = slotsKinds[slot.Kind]
	if !ok {
		return nil, errgo.Newf("unknown slot kind %q", slot.Kind)
	}
	var err error
	ctlSlot.Start, err = time.ParseDuration(slot.Start)
	if err != nil || ctlSlot.Start < 0 || ctlSlot.Start >= 24*time.Hour {
		return nil, errgo.Newf("invalid start time %q", slot.Start)
	}
	ctlSlot.SlotDuration, err = time.ParseDuration(slot.Duration)
	if err != nil || ctlSlot.Duration < 0 || ctlSlot.Duration >= 24*time.Hour {
		return nil, errgo.Newf("invalid duration %q", slot.Duration)
	}
	ctlSlot.Duration, err = time.ParseDuration(slot.Duration)
	if err != nil || ctlSlot.Duration < 0 || ctlSlot.Duration >= 24*time.Hour {
		return nil, errgo.Newf("invalid discretionary duration %q", slot.Duration)
	}
	return &ctlSlot, nil
}

func slotOverlap(s0, s1 *hydroctl.Slot) bool {
	// The time is cyclic, so first swap the slots so the one
	// that starts earliest is first.
	if s0.Start > s1.Start {
		s0, s1 = s1, s0
	}
	s0end, s1end := s0.Start+s0.Duration, s1.Start+s1.Duration
	if s0.Start < s1end && s0end >= s1.Start {
		return true
	}
	// Try with s1 24 hours offset.
	s1start := s1.Start + 24*time.Hour
	s1end = s1start + s1.Duration
	if s0.Start < s1end && s0end >= s1start {
		return true
	}
	// Try with s1 24 hours offset the other way
	// TODO is this actually necessary?
	s1start = s1.Start - 24*time.Hour
	s1end = s1start + s1.Duration
	if s0.Start < s1end && s0end >= s1start {
		return true
	}
	return false
}

var modes = map[string]hydroctl.RelayMode{
	"off":        hydroctl.AlwaysOff,
	"on":         hydroctl.AlwaysOn,
	"in-use":     hydroctl.InUse,
	"not-in-use": hydroctl.NotInUse,
}

func parsePower(p string) (int, error) {
	prefix := p
	suffix := ""
	for i, c := range p {
		if c < '0' || c > '9' {
			prefix, suffix = p[:i], p[i:]
			break
		}
	}
	suffix = strings.ToLower(suffix)
	n, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, errgo.Newf("invalid power %q", p)
	}
	switch suffix {
	case "", "w":
		return n, nil
	case "kw":
		return n * 1000, nil
	}
	return 0, errgo.Newf("invalid power %q", p)
}

package hydroconfig

import (
	"fmt"
	"github.com/rogpeppe/hydro/hydroctl"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Config represents a control system configuration as specified
// by the user.
type Config struct {
	Cohorts []Cohort
}

// Cohort represents a configured set of relays associated with the
// same rule.
type Cohort struct {
	Name          string
	Relays        []int
	MaxPower      int
	Mode          hydroctl.RelayMode
	InUseSlots    []*hydroctl.Slot
	NotInUseSlots []*hydroctl.Slot
}

// CtlConfig returns the hydroctl configuration that derives
// from c. It ignores duplicate and out-of-range relays.
func (c *Config) CtlConfig() *hydroctl.Config {
	relays := make([]hydroctl.RelayConfig, hydroctl.MaxRelayCount)
	found := make([]bool, hydroctl.MaxRelayCount)
	for _, cohort := range c.Cohorts {
		for _, r := range cohort.Relays {
			if r < 0 || r >= hydroctl.MaxRelayCount || found[r] {
				// TODO log?
				continue
			}
			found[r] = true
			relays[r] = hydroctl.RelayConfig{
				Mode:     cohort.Mode,
				MaxPower: cohort.MaxPower,
				InUse:    cohort.InUseSlots,
				NotInUse: cohort.NotInUseSlots,
				Cohort:   cohort.Name,
			}
		}
	}
	return &hydroctl.Config{
		Relays: relays,
	}
}

// Parse parses the contents of a hydro configuration file.
//
// A sample config:
//
//	relay 6 is dining room
//	relays 0, 4, 5 are bedrooms
//
//	dining room on from 14:30 to 20:45 for at least 20m
//	bedrooms on from 17:00 to 20:00
func Parse(s string) (*Config, error) {
	// TODO in use/not in use
	// TODO maxpower
	p := &configParser{
		assignedRelays: make(map[int]string),
	}
	for t := newText(s); t.s != ""; {
		var line text
		line, t = t.line()
		p.addLine(line)
	}
	if len(p.errors) > 0 {
		return nil, &ConfigParseError{
			Config: s,
			Errors: p.errors,
		}
	}
	sort.Sort(cohortsByName(p.cohorts))
	return &Config{
		Cohorts: p.cohorts,
	}, nil
}

type configParser struct {
	cohorts []Cohort
	errors  []ParseError
	// assignedRelays maps relay numbers to the
	// cohort name that the relay is assigned to.
	assignedRelays map[int]string
}

func (p *configParser) addLine(t text) {
	word, rest := t.word()
	if word.s == "" {
		return
	}
	// "relay 6 is dining room"
	// "relays 0, 4, 5 are bedrooms"
	if word.eqFold("relay") || word.eqFold("relays") {
		p.addCohort(rest)
		return
	}

	// "dining room on from 14:30 to 20:45 for at least 20m"
	// "bedrooms on from 17:00 to 20:00"
	var found *Cohort
	for i := range p.cohorts {
		c := &p.cohorts[i]
		if rest, ok := t.trimPrefix(c.Name); ok {
			found = c
			t = rest
			break
		}
	}
	if found == nil {
		p.errorf(t, "unrecognised line")
		return
	}
	if slot := p.parseSlot(t); slot != nil {
		found.InUseSlots = append(found.InUseSlots, slot)
	}
}

func (p *configParser) parseSlot(t text) *hydroctl.Slot {
	// "on from 14:30 to 20:45 for at least 20m"
	// "on from 17:00 to 20:00"
	word, rest := t.word()
	if word.s == "on" {
		t = rest
	}
	word, rest = t.word()
	if word.s != "from" {
		p.errorf(t, "expected 'from'")
		return nil
	}
	var slot hydroctl.Slot
	t = rest
	startTime, t, ok := p.parseTime(t)
	if !ok {
		return nil
	}
	slot.Start = startTime

	word, rest = t.word()
	if word.s != "to" {
		p.errorf(t, "expected 'to'")
		return nil
	}
	t = rest
	endTime, t, ok := p.parseTime(t)
	if !ok {
		return nil
	}
	if endTime < startTime {
		endTime += 24 * time.Hour
	}
	slot.SlotDuration = endTime - startTime
	slot.Duration = endTime - startTime
	slot.Kind = hydroctl.Exactly

	if word, _ := t.word(); word.s == "" {
		return &slot
	}

	if rest, ok = t.trimPrefix("for at most"); ok {
		slot.Kind = hydroctl.AtMost
		t = rest
	} else if rest, ok = t.trimPrefix("for at least"); ok {
		slot.Kind = hydroctl.AtLeast
		t = rest
	} else if rest, ok = t.trimPrefix("for"); ok {
		t = rest
	} else {
		p.errorf(word, "expected 'for', 'for at least' or 'for at most'")
		return nil
	}
	word, rest = t.word()
	if word.s == "" {
		p.errorf(t, "expected duration")
		return nil
	}
	dur, err := time.ParseDuration(word.s)
	if err != nil {
		p.errorf(t, "invalid duration: %v", err)
		return nil
	}
	t = rest
	slot.Duration = dur
	if word, _ := t.word(); word.s != "" {
		p.errorf(word, "unexpected extra text")
		return nil
	}
	return &slot
}

var timeFormats = []string{
	"15:04",
	"15:04",
	"3pm",
	"3:04pm",
}

func (p *configParser) parseTime(t text) (time.Duration, text, bool) {
	word, rest := t.word()
	if word.s == "" {
		return 0, text{}, false
	}
	for _, f := range timeFormats {
		if t, err := time.Parse(f, word.s); err == nil {
			return time.Duration(t.Hour())*time.Hour +
					time.Duration(t.Minute())*time.Minute +
					time.Duration(t.Second())*time.Second,
				rest,
				true
		}
	}
	p.errorf(t, "invalid time value %q", word.s)
	return 0, text{}, false
}

func (p *configParser) addCohort(t text) {
	// "1 is dining room"
	// "2, 3, 4 are bedrooms"

	whole := t
	var relays []int
	for {
		word, rest := t.word()
		if word.s == "" {
			p.errorf(t, "expected relay number")
			return
		}
		t = rest
		if word.s == "is" || word.s == "are" {
			break
		}
		s := strings.TrimSuffix(word.s, ",")
		if s == "" {
			continue
		}
		relay, err := strconv.Atoi(s)
		if err != nil {
			p.errorf(word, "invalid relay number")
			continue
		}
		if relay < 0 || relay >= hydroctl.MaxRelayCount {
			p.errorf(word, "relay number out of bounds")
			continue
		}
		relays = append(relays, relay)
	}
	name := t.trimSpace()
	if name.s == "" {
		p.errorf(whole, "empty cohort name")
		return
	}
	for _, c := range p.cohorts {
		if strings.EqualFold(c.Name, name.s) {
			p.errorf(name, "duplicate cohort name")
			return
		}
	}
	for _, relay := range relays {
		if dupe, ok := p.assignedRelays[relay]; ok {
			// TODO error with actual relay text.
			p.errorf(t, "duplicate relay %d also in %q", relay, dupe)
		}
		p.assignedRelays[relay] = name.s
	}
	p.cohorts = append(p.cohorts, Cohort{
		Name:   name.s,
		Mode:   hydroctl.InUse,
		Relays: relays,
	})
}

func (p *configParser) errorf(t text, f string, a ...interface{}) {
	p.errors = append(p.errors, ParseError{
		P0:      t.p0,
		P1:      t.p1,
		Message: fmt.Sprintf(f, a...),
	})
}

type ConfigParseError struct {
	Config string
	Errors []ParseError
}

type ParseError struct {
	P0, P1  int
	Message string
}

func (e *ConfigParseError) Error() string {
	m := fmt.Sprintf("error at %q: %v", e.Config[e.Errors[0].P0:e.Errors[0].P1], e.Errors[0].Message)
	if len(e.Errors) > 1 {
		m += fmt.Sprintf(" (and %d more)", len(e.Errors)-1)
	}
	return m
}

type cohortsByName []Cohort

func (c cohortsByName) Less(i, j int) bool {
	return c[i].Name < c[j].Name
}

func (c cohortsByName) Len() int {
	return len(c)
}

func (c cohortsByName) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

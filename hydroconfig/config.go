package hydroconfig

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/hydroctl"
)

// Config represents a control system configuration as specified
// by the user.
type Config struct {
	Cohorts []Cohort
	Relays  map[int]Relay
}

// Relay holds information specific to a relay.
type Relay struct {
	MaxPower int // maximum power that this relay can draw in watts.
}

// Cohort represents a configured set of relays associated with the
// same rule.
type Cohort struct {
	Name          string
	Relays        []int
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
				MaxPower: c.Relays[r].MaxPower,
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
// On error it returns a *ConfigParseError containing
// any errors found.
//
// A sample config:
//
//	relay 6 is dining room
//	relays 0, 4, 5 are br (bedrooms)
//
//	relay 4 has max power 300w
//	relays 0, 7, 8 have max power 5kw
//
//	dining room on from 14:30 to 20:45 for at least 20m
//	bedrooms on from 17:00 to 20:00
//
// If the time range is omitted, the slot lasts all day.
func Parse(s string) (*Config, error) {
	// TODO in use/not in use
	// TODO maxpower
	p := &configParser{
		relayInfo:      make(map[int]Relay),
		assignedRelays: make(map[int]string),
		shortNames:     make(map[string]int),
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
	for i := range p.cohorts {
		cohort := &p.cohorts[i]
		// TODO what should we do when we implement not-in-use support?
		// The AlwaysOn mode doesn't seem to make much sense then, perhaps.
		if len(cohort.InUseSlots) == 1 && *cohort.InUseSlots[0] == allDaySlot {
			cohort.InUseSlots = nil
			cohort.Mode = hydroctl.AlwaysOn
		}
	}
	sort.Sort(cohortsByName(p.cohorts))
	if len(p.relayInfo) == 0 {
		// Make tests a little easier.
		p.relayInfo = nil
	}
	return &Config{
		Cohorts: p.cohorts,
		Relays:  p.relayInfo,
	}, nil
}

type configParser struct {
	cohorts []Cohort
	errors  []ParseError
	// assignedRelays maps relay numbers to the
	// cohort name that the relay is assigned to.
	assignedRelays map[int]string
	relayInfo      map[int]Relay
	shortNames     map[string]int
}

func (p *configParser) addLine(t text) {
	t = t.trimSpace()
	// Ignore comment lines.
	if strings.HasPrefix(t.s, "#") {
		return
	}
	// Trim off any final full stop.
	if strings.HasSuffix(t.s, ".") {
		t = t.slice(0, len(t.s)-1)
	}
	word, rest := t.word()
	if word.s == "" {
		return
	}
	// "relay 6 is dining room"
	// "relays 0, 4, 5 are bedrooms"
	// "relay 5 has max power 500w"
	// "relays 0, 4, 5 have max power 2kw"
	if word.eq("relay") || word.eq("relays") {
		p.addCohortOrMaxPower(rest)
		return
	}

	// "dining room on from 14:30 to 20:45 for at least 20m"
	// "bedrooms on from 17:00 to 20:00"
	var found *Cohort
	for shortName, index := range p.shortNames {
		if rest, ok := t.trimPrefix(shortName); ok {
			found = &p.cohorts[index]
			t = rest
			break
		}
	}
	if found == nil {
		for i := range p.cohorts {
			c := &p.cohorts[i]
			if rest, ok := t.trimPrefix(c.Name); ok {
				found = c
				t = rest
				break
			}
		}
	}
	if found == nil {
		p.errorf(t, "line must start with 'relay' or relay cohort name")
		return
	}
	if slot := p.parseSlot(t); slot != nil {
		for _, oldSlot := range found.InUseSlots {
			if oldSlot.Overlaps(slot) {
				// TODO format this with proper-looking times.
				p.errorf(t, "time slot overlaps %v slot from %v", oldSlot.SlotDuration, oldSlot.Start)
				return
			}
		}
		found.InUseSlots = append(found.InUseSlots, slot)
	}
}

var allDaySlot = hydroctl.Slot{
	Start:        0,
	SlotDuration: 24 * time.Hour,
	Kind:         hydroctl.Exactly,
	Duration:     24 * time.Hour,
}

func (p *configParser) parseSlot(t text) *hydroctl.Slot {
	// "on from 14:30 to 20:45 for at least 20m"
	// "on from 17:00 to 20:00"
	// "is on from..."
	// "are on from..."
	// "is on"
	// "are on"
	// "on for at least 20m"

	t, ok := t.trimWord("is")
	if !ok {
		t, ok = t.trimWord("are")
	}
	// Technically "foo is from 12pm..." doesn't read well, but it's easier to allow it.
	t, _ = t.trimWord("on")

	slot := allDaySlot
	word, rest := t.word()
	if word.s == "from" {
		var startTime, endTime time.Duration
		var ok bool
		t = rest
		startTime, t, ok = p.parseTime(t)
		if !ok {
			return nil
		}
		slot.Start = startTime

		word, rest = t.word()
		if word.s != "to" {
			p.errorf(word, "expected 'to'")
			return nil
		}
		t = rest
		endTime, t, ok = p.parseTime(t)
		if !ok {
			return nil
		}
		if endTime < startTime {
			endTime += 24 * time.Hour
		}
		slot.SlotDuration = endTime - startTime
		slot.Duration = endTime - startTime
	}
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
	p.errorf(word, "invalid time value %q. Can use 15:04, 3pm, 3:04pm.", word.s)
	return 0, text{}, false
}

func (p *configParser) addCohortOrMaxPower(t text) {
	// "1 is dining room"
	// "2, 3, 4 are bedrooms"

	whole := t
	var relays []int
	isNewCohort := false
relayNumbers:
	for {
		word, rest := t.word()
		if word.s == "" {
			p.errorf(t, "expected relay number, got %q", word.s)
			return
		}
		t = rest
		switch {
		case word.eq("is"), word.eq("are"):
			isNewCohort = true
			break relayNumbers
		case word.eq("has"), word.eq("have"):
			if rest, ok := t.trimPrefix("max power"); ok {
				t = rest
				break relayNumbers
			}
			if rest, ok := t.trimPrefix("maximum power"); ok {
				t = rest
				break relayNumbers
			}
			if rest, ok := t.trimPrefix("maxpower"); ok {
				t = rest
				break relayNumbers
			}
			p.errorf(t, "expected max power setting")
			return
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
	if isNewCohort {
		p.addCohort(t, relays)
		return
	}
	word, rest := t.word()
	if word.s == "" {
		p.errorf(t, "expected power value")
		return
	}
	t = rest
	watts, err := parsePower(word.s)
	if err != nil {
		p.errorf(t, "bad power value: %v", err)
		return
	}
	word, rest = t.word()
	if word.s != "" {
		p.errorf(t, "unexpected text after power value")
		return
	}
	for _, r := range relays {
		if _, ok := p.assignedRelays[r]; !ok {
			p.errorf(whole, "unassigned relay %d", r)
			return
		}
		info := p.relayInfo[r]
		info.MaxPower = watts
		p.relayInfo[r] = info
	}
}

func parsePower(s string) (int, error) {
	i := strings.LastIndexFunc(s, isDigit)
	if i == -1 {
		return 0, errgo.New("no digits")
	}
	num, suffix := s[0:i+1], s[i+1:]
	n, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, errgo.New("bad number")
	}
	if n < 0 {
		return 0, errgo.New("negative power")
	}
	m := 1.0
	switch strings.ToLower(suffix) {
	case "w":
	case "kw":
		m = 1e3
	case "mw":
		m = 1e6
	default:
		return 0, errgo.New("unknown power unit")
	}
	return int(m*n + 0.5), nil
}

func isDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

func (p *configParser) addCohort(t text, relays []int) {
	name := t.trimSpace()
	shortName := name
	if i := strings.Index(name.s, "("); i != -1 {
		name = name.slice(i+1, len(name.s))
		if strings.HasSuffix(name.s, ")") {
			name = name.slice(0, len(name.s)-1)
		}
		shortName = shortName.slice(0, i).trimSpace()
	}
	if shortName.s == "" {
		p.errorf(shortName, "empty cohort name")
		return
	}
	if name.s == "" {
		p.errorf(name, "empty cohort name")
	}
	for _, c := range p.cohorts {
		if strings.EqualFold(c.Name, name.s) {
			p.errorf(name, "duplicate cohort name")
			return
		}
	}
	for s := range p.shortNames {
		if strings.EqualFold(shortName.s, s) {
			p.errorf(shortName, "duplicate cohort name")
		}
	}
	for _, relay := range relays {
		if dupe, ok := p.assignedRelays[relay]; ok {
			// TODO error with actual relay text.
			p.errorf(t, "duplicate relay %d also in %q", relay, dupe)
		}
		p.assignedRelays[relay] = name.s
	}
	if name != shortName {
		p.shortNames[shortName.s] = len(p.cohorts)
	}
	p.cohorts = append(p.cohorts, Cohort{
		Name:   name.s,
		Mode:   hydroctl.InUse,
		Relays: relays,
	})
}

func isSpaceOrDigit(r rune) bool {
	return unicode.IsSpace(r) || '0' <= r && r <= '9'
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

// ParseError holds a single parse error.
type ParseError struct {
	// [P0, P1] is the range of text the error pertains to.
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

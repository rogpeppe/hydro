package hydroctl

import (
	"fmt"
	"time"
)

// TimeOfDay holds an absolute time within a day.
type TimeOfDay struct {
	d time.Duration
}

// Hour returns the hour from 0-23.
func (t TimeOfDay) Hour() int {
	return int(t.d / time.Hour)
}

// Minute returns the minute from 0-59.
func (t TimeOfDay) Minute() int {
	return int(t.d / time.Minute % 60)
}

// Second returns the second from 0-59 (currently this is always zero).
func (t TimeOfDay) Second() int {
	return int(t.d / time.Second % 60)
}

func (t TimeOfDay) String() string {
	return fmt.Sprintf("%.2d:%.2d", t.d/time.Hour, (t.d/time.Minute)%60)
}

func (t TimeOfDay) Before(t1 TimeOfDay) bool {
	return t.d < t1.d
}

func (t TimeOfDay) After(t1 TimeOfDay) bool {
	return t.d > t1.d
}

func (t TimeOfDay) Equal(t1 TimeOfDay) bool {
	return t == t1
}

var timeFormats = []string{
	"15:04",
	"3pm",
	"3:04pm",
}

// ParseTimeOfDay parses a time of day in on of the formats 15:05, 3pm or 3:04pm.
func ParseTimeOfDay(s string) (TimeOfDay, error) {
	for _, f := range timeFormats {
		if t, err := time.Parse(f, s); err == nil {
			return TimeOfDayFromTime(t), nil
		}
	}
	return TimeOfDay{}, fmt.Errorf("invalid time of day value %q. Can use 15:04, 3pm, 3:04pm.", s)
}

// TimeOfDay returns the time of day of the given time instance.
func TimeOfDayFromTime(t time.Time) TimeOfDay {
	return TimeOfDay{
		d: time.Duration(t.Hour())*time.Hour +
			time.Duration(t.Minute())*time.Minute +
			time.Duration(t.Second())*time.Second,
	}
}

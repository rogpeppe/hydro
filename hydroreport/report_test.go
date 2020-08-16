package hydroreport

import (
	"bytes"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func TestWriteReport(t *testing.T) {
	c := qt.New(t)
	/*
		Test timeline:

		time	description					generator here 	neighbour
		0h	nothing happening				0		0		0
		8h	generator exporting				50000	0		0
		10h	here using exported				50000	10000	0
		12h	neighbour using exported		50000	10000	5000
		14h	here importing					50000	60000	5000
		16h	neighbour importing				50000	15000	70000
		18h	both importing					50000	60000	70000
	*/
	generatorSamples := NewMemSampleReader([]Sample{{
		Time:        epoch,
		TotalEnergy: 0,
	}, {
		Time:        epoch.Add(8 * time.Hour),
		TotalEnergy: 0,
	}, {
		Time:        epoch.Add(48 * time.Hour),
		TotalEnergy: 50000 * 40,
	}})
	hereSamples := NewMemSampleReader([]Sample{{
		Time:        epoch,
		TotalEnergy: 0,
	}, {
		Time:        epoch.Add(10 * time.Hour),
		TotalEnergy: 0,
	}, {
		Time:        epoch.Add(14 * time.Hour),
		TotalEnergy: 10000 * 4,
	}, {
		Time:        epoch.Add(16 * time.Hour),
		TotalEnergy: 10000*4 + 60000*2,
	}, {
		Time:        epoch.Add(18 * time.Hour),
		TotalEnergy: 10000*4 + 60000*2 + 15000*2,
	}, {
		Time:        epoch.Add(40 * time.Hour),
		TotalEnergy: 10000*4 + 60000*2 + 15000*2 + 60000*(40-18),
	}})
	neighbourSamples := NewMemSampleReader([]Sample{{
		Time:        epoch,
		TotalEnergy: 0,
	}, {
		Time:        epoch.Add(12 * time.Hour),
		TotalEnergy: 0,
	}, {
		Time:        epoch.Add(16 * time.Hour),
		TotalEnergy: 5000 * 4,
	}, {
		Time:        epoch.Add(46 * time.Hour),
		TotalEnergy: 5000*4 + (46-16)*70000,
	}})

	var buf bytes.Buffer
	err := WriteReport(&buf, ReportParams{
		Generator: NewUsageReader(generatorSamples, epoch, time.Minute),
		Here:      NewUsageReader(hereSamples, epoch, time.Minute),
		Neighbour: NewUsageReader(neighbourSamples, epoch, time.Minute),
		EndTime:   epoch.Add(24 * time.Hour),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(buf.String(), qt.Equals, `
Time,Export to grid (kWH),Export power used by Aliday (kWH),Export power used by Drynoch (kWH),Import power used by Aliday (kWH),Import power used by Drynoch (kWH)
2000-01-02 12:00 UTC,0,0,0,0,0
2000-01-02 13:00 UTC,0,0,0,0,0
2000-01-02 14:00 UTC,0,0,0,0,0
2000-01-02 15:00 UTC,0,0,0,0,0
2000-01-02 16:00 UTC,0,0,0,0,0
2000-01-02 17:00 UTC,0,0,0,0,0
2000-01-02 18:00 UTC,0,0,0,0,0
2000-01-02 19:00 UTC,0,0,0,0,0
2000-01-02 20:00 UTC,50,0,0,0,0
2000-01-02 21:00 UTC,50,0,0,0,0
2000-01-02 22:00 UTC,40,0,10,0,0
2000-01-02 23:00 UTC,40,0,10,0,0
2000-01-03 00:00 UTC,35,5,10,0,0
2000-01-03 01:00 UTC,35,5,10,0,0
2000-01-03 02:00 UTC,0,5,45,0,15
2000-01-03 03:00 UTC,0,5,45,0,15
2000-01-03 04:00 UTC,0,35,15,35,0
2000-01-03 05:00 UTC,0,35,15,35,0
2000-01-03 06:00 UTC,0,25,25,43,37
2000-01-03 07:00 UTC,0,25,25,43,37
2000-01-03 08:00 UTC,0,25,25,43,37
2000-01-03 09:00 UTC,0,25,25,43,37
2000-01-03 10:00 UTC,0,25,25,43,37
2000-01-03 11:00 UTC,0,25,25,43,37
`[1:])
}
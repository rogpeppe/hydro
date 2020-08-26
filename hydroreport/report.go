package hydroreport

import (
	"fmt"
	"io"
	"math"
	"time"

	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/meterstat"
)

// Params holds parameters for the Open function.
type Params struct {
	// The UsageReaders hold the usage readers
	// for the meters that the report takes into account.
	// They must all start at the same instance, have the same quantum
	// and provide usage information at least until after the end time.
	// Additionally, the quantum must evenly divide an hour.
	Generator meterstat.UsageReader
	Neighbour meterstat.UsageReader
	Here      meterstat.UsageReader
	// EndTime holds the time that the report will end (not inclusive).
	// It must be a whole hour multiple.
	EndTime time.Time
	// TZ holds the time zone to use for the report times.
	// If it's nil, time.UTC will be used.
	TZ *time.Location
}

// Entry holds a entry line in a report, corresponding to 1 hour of readings.
type Entry struct {
	Time time.Time
	hydroctl.PowerChargeable
}

// Reader represents a reader of report entry lines.
type Reader interface {
	ReadEntry() (Entry, error)
}

// Open returns a reader that reads entries from the report.
func Open(p Params) (Reader, error) {
	if p.TZ == nil {
		p.TZ = time.UTC
	}
	p.EndTime = p.EndTime.In(p.TZ)
	if err := checkUsageReaderConsistency(
		p.Generator,
		p.Neighbour,
		p.Here,
	); err != nil {
		return nil, fmt.Errorf("inconsistent usage readers: %v", err)
	}
	if !wholeHour(p.EndTime) {
		return nil, fmt.Errorf("report end time %s is not on a whole hour", p.EndTime)
	}
	t := p.Generator.Time().In(p.TZ)
	if !wholeHour(t) {
		return nil, fmt.Errorf("report start time %s is not on a whole hour", t)
	}
	quantum := p.Generator.Quantum()
	if time.Hour%quantum != 0 {
		return nil, fmt.Errorf("usage reader quantum %v does not divide an hour evenly", quantum)
	}
	return &reportReader{
		currentTime:       t,
		quantum:           quantum,
		samplesPerQuantum: int(time.Hour / quantum),
		p:                 p,
	}, nil
}

type reportReader struct {
	currentTime       time.Time
	samplesPerQuantum int
	quantum           time.Duration
	p                 Params
}

// ReadEntry implements Reader.
func (r *reportReader) ReadEntry() (Entry, error) {
	if !r.currentTime.Before(r.p.EndTime) {
		return Entry{}, io.EOF
	}
	var total hydroctl.PowerChargeable
	for i := 0; i < r.samplesPerQuantum; i++ {
		var pu hydroctl.PowerUse

		u, err := r.p.Generator.ReadUsage()
		if err != nil {
			return Entry{}, fmt.Errorf("generator usage samples stopped early (at %v): %v", r.p.Generator.Time(), err)
		}
		pu.Generated = u.Energy

		u, err = r.p.Neighbour.ReadUsage()
		if err != nil {
			return Entry{}, fmt.Errorf("neighbour usage samples stopped early (at %v): %v", r.p.Neighbour.Time(), err)
		}
		pu.Neighbour = u.Energy

		u, err = r.p.Here.ReadUsage()
		if err != nil {
			return Entry{}, fmt.Errorf("here usage samples stopped early (at %v): %v", r.p.Here.Time(), err)
		}
		pu.Here = u.Energy

		total = total.Add(hydroctl.ChargeablePower(pu))
	}
	rec := Entry{
		PowerChargeable: total,
		Time:            r.currentTime,
	}
	r.currentTime = r.currentTime.Add(time.Hour)
	return rec, nil
}

// Write writes a report with entries read from r.
func Write(w io.Writer, r Reader) error {
	fmt.Fprintln(w, "Time,"+
		"Export to grid (kWH),"+
		// TODO don't hard-code the names!
		"Export power used by Aliday (kWH),"+
		"Export power used by Drynoch (kWH),"+
		"Import power used by Aliday (kWH),"+
		"Import power used by Drynoch (kWH)",
	)
	for {
		rec, err := r.ReadEntry()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		fmt.Fprintf(w, "%v,%s,%s,%s,%s,%s\n",
			rec.Time.Format("2006-01-02 15:04 MST"),
			powerStr(rec.ExportGrid),
			powerStr(rec.ExportNeighbour),
			powerStr(rec.ExportHere),
			powerStr(rec.ImportNeighbour),
			powerStr(rec.ImportHere),
		)
	}
}

func powerStr(f float64) string {
	return fmt.Sprintf("%.3f", math.RoundToEven(f)/1000)
}

func wholeHour(t time.Time) bool {
	return t.Truncate(time.Hour).Equal(t)
}

func checkUsageReaderConsistency(rs ...meterstat.UsageReader) error {
	if len(rs) == 0 {
		return fmt.Errorf("no UsageReaders provided")
	}
	startTime := rs[0].Time()
	quantum := rs[0].Quantum()
	for _, r := range rs {
		if !r.Time().Equal(startTime) {
			return fmt.Errorf("inconsistent start time")
		}
		if r.Quantum() != quantum {
			return fmt.Errorf("inconsistent quantum")
		}
	}
	return nil
}

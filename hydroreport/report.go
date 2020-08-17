package hydroreport

import (
	"fmt"
	"io"
	"math"
	"time"

	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/meterstat"
)

type ReportParams struct {
	// The UsageReaders hold the usage readers
	// for the meters that the report takes into account.
	// They must all start at the same instance, have the same quantum
	// and provide usage information at least until after the end time.
	// Additionally, the quantum must evenly divide an hour.
	Generator meterstat.UsageReader
	Neighbour meterstat.UsageReader
	Here      meterstat.UsageReader
	// EndTime holds the time that the report will end.
	// It must be a whole hour multiple.
	EndTime time.Time
	// Location holds the time zone to use for the report times.
	// If it's nil, time.UTC will be used.
	Location *time.Location
}

// WriteReport writes a CSV file containing an hour-by-hour report of
// energy usage over the course of a month.
func WriteReport(w io.Writer, p ReportParams) error {
	if p.Location == nil {
		p.Location = time.UTC
	}
	p.EndTime = p.EndTime.In(p.Location)
	if err := checkUsageReaderConsistency(
		p.Generator,
		p.Neighbour,
		p.Here,
	); err != nil {
		return fmt.Errorf("inconsistent usage readers: %v", err)
	}
	if !wholeHour(p.EndTime) {
		return fmt.Errorf("report end time %s is not on a whole hour", p.EndTime)
	}
	t := p.Generator.Time().In(p.Location)
	if !wholeHour(t) {
		return fmt.Errorf("report start time %s is not on a whole hour", t)
	}
	quantum := p.Generator.Quantum()
	if time.Hour%quantum != 0 {
		return fmt.Errorf("usage reader quantum %v does not divide an hour evenly", quantum)
	}
	fmt.Fprintln(w, "Time,"+
		"Export to grid (kWH),"+
		// TODO don't hard-code the names!
		"Export power used by Aliday (kWH),"+
		"Export power used by Drynoch (kWH),"+
		"Import power used by Aliday (kWH),"+
		"Import power used by Drynoch (kWH)",
	)
	var total hydroctl.PowerChargeable
	samples := 0
	for {
		if wholeHour(t) && samples > 0 {
			fmt.Fprintf(w, "%v,%s,%s,%s,%s,%s\n",
				t.Add(-time.Hour).In(p.Location).Format("2006-01-02 15:04 MST"),
				powerStr(total.ExportGrid),
				powerStr(total.ExportNeighbour),
				powerStr(total.ExportHere),
				powerStr(total.ImportNeighbour),
				powerStr(total.ImportHere),
			)
			total = hydroctl.PowerChargeable{}
		}
		if t.Add(quantum).After(p.EndTime) {
			break
		}
		var err error
		var pu hydroctl.PowerUse
		pu.Generated, err = p.Generator.ReadUsage()
		if err != nil {
			return fmt.Errorf("generator usage samples stopped early (at %v): %v", t, err)
		}
		pu.Neighbour, err = p.Neighbour.ReadUsage()
		if err != nil {
			return fmt.Errorf("neighbour usage samples stopped early (at %v): %v", t, err)
		}
		pu.Here, err = p.Here.ReadUsage()
		if err != nil {
			return fmt.Errorf("here usage samples stopped early (at %v): %v", t, err)
		}
		cp := hydroctl.ChargeablePower(pu)
		total.ExportGrid += cp.ExportGrid
		total.ExportNeighbour += cp.ExportNeighbour
		total.ExportHere += cp.ExportHere
		total.ImportNeighbour += cp.ImportNeighbour
		total.ImportHere += cp.ImportHere
		samples++
		t = t.Add(quantum)
	}
	return nil
}

func powerStr(f float64) string {
	return fmt.Sprintf("%.3f", math.RoundToEven(f)/1000)
}

func wholeHour(t time.Time) bool {
	return t.Truncate(time.Hour).Equal(t)
}

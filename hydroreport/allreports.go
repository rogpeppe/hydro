package hydroreport

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/rogpeppe/hydro/meterstat"
)

//go:generate stringer -trimprefix Loc -type MeterLocation

type MeterLocation int

const (
	LocUnknown MeterLocation = iota
	LocGenerator
	LocNeighbour
	LocHere
)

var future = time.Date(3000, time.January, 1, 0, 0, 0, 0, time.UTC)

type AllReportsParams struct {
	// SampleDir holds the directory holding all the meter directories.
	SampleDir string
	// Meters holds the set of meter names for each meter
	// location (typically this contains an identifier for the meter).
	// The data for that meter is assumed to be in the directory $dir/$name
	// in any file named *.sample.
	//
	// Invalid sample files will be ignored.
	Meters map[MeterLocation][]string
	// TZ holds the time zone to use for the generated reports
	// (UTC if it's nil)
	TZ *time.Location
}

// AllReports returns a slice containing an element for each possible monthly report that can be
// made from the data in the given directory.
//
// A report can only be generated for a given month if there's sample data
// spanning that month for all specified meters.
func AllReports(p AllReportsParams) ([]*Report, error) {
	if len(p.Meters) != 3 {
		return nil, fmt.Errorf("missing meter names for some meter locations (got %v)", p.Meters)
	}
	if p.TZ == nil {
		p.TZ = time.UTC
	}
	// t0loc holds the latest start time of for each of a given location's sample directories.
	t0loc := make(map[MeterLocation]time.Time)
	// t0loc holds the earliest end time of for each of a given location's sample directories.
	t1loc := make(map[MeterLocation]time.Time)
	meterDirs := make(map[MeterLocation][]*meterstat.MeterSampleDir)
	for location, names := range p.Meters {
		for _, name := range names {
			meterDir := filepath.Join(p.SampleDir, name)
			sd, err := meterstat.ReadSampleDir(meterDir, "*.sample")
			if err != nil {
				return nil, fmt.Errorf("cannot read sample dir %v: %v", meterDir, err)
			}
			meterDirs[location] = append(meterDirs[location], sd)
			t0, ok := t0loc[location]
			if !ok || sd.T0.After(t0) {
				t0loc[location] = sd.T0
			}
			t1, ok := t1loc[location]
			if !ok || sd.T1.Before(t1) {
				t1loc[location] = sd.T1
			}
		}
	}
	// Determine the full possible range of samples.
	t0 := future
	var t1 time.Time
	for _, t := range t0loc {
		if t.Before(t0) {
			t0 = t
		}
	}
	for _, t := range t1loc {
		if t.After(t1) {
			t1 = t
		}
	}
	// Find out what reports are possible.
	var reports []*Report
	for year := t0.Year(); year <= t1.Year(); year++ {
		for month := time.January; month <= time.December; month++ {
			t0r := time.Date(year, month, 1, 0, 0, 0, 0, p.TZ)
			t1r := t0r.AddDate(0, 1, 0)
			// We can generate a report if we've got encompassing samples
			// for all the meter locations.
			ok := true
			for location := range meterDirs {
				if t0r.Before(t0loc[location]) || t1r.After(t1loc[location]) {
					// The location samples don't cover this month.
					ok = false
					break
				}
			}
			if ok {
				reports = append(reports, &Report{
					MeterDirs: meterDirs,
					T0:        t0r,
					T1:        t1r,
					tz:        p.TZ,
				})
			}
		}
	}
	return reports, nil
}

// Report represents a report generated from available samples.
type Report struct {
	// MeterDirs holds all the directories containing the samples
	// indexed by meter location.
	MeterDirs map[MeterLocation][]*meterstat.MeterSampleDir
	// T0 and T1 hold the time range of the report.
	T0, T1 time.Time
	tz     *time.Location
}

// Params returns the parameters for WriteReport.
func (r Report) Params() ReportParams {
	locUsageReaders := make(map[MeterLocation]meterstat.UsageReader)
	for loc, sds := range r.MeterDirs {
		usageReaders := make([]meterstat.UsageReader, 0, len(sds))
		for _, sd := range sds {
			usageReaders = append(usageReaders, meterstat.NewUsageReader(sd.OpenRange(r.T0, r.T1), r.T0, time.Minute))
		}
		locUsageReaders[loc] = meterstat.SumUsage(usageReaders...)
	}
	return ReportParams{
		Generator: locUsageReaders[LocGenerator],
		Neighbour: locUsageReaders[LocNeighbour],
		Here:      locUsageReaders[LocHere],
		EndTime:   r.T1,
		TZ:        r.tz,
	}
}

// Write writes the report as a CSV to w.
func (r *Report) Write(w io.Writer) error {
	return WriteReport(w, r.Params())
}

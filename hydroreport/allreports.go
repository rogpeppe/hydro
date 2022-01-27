package hydroreport

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/rogpeppe/hydro/meterstat"
)

//go:generate go run golang.org/x/tools/cmd/stringer@v0.1.9 -trimprefix Loc -type MeterLocation

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
// A report can only be generated for a given month if there's some sample data
// within that month for all specified meters. If the entire month isn't
// covered, the report will be labeled as "partial".
func AllReports(p AllReportsParams) ([]*Report, error) {
	if len(p.Meters) != 3 {
		return nil, fmt.Errorf("missing meter names for some meter locations (got %v)", p.Meters)
	}
	if p.TZ == nil {
		p.TZ = time.UTC
	}
	// locRange holds the possible time range for each location.
	locRange := make(map[MeterLocation]meterstat.TimeRange)
	meterDirs := make(map[MeterLocation][]*meterstat.MeterSampleDir)
	totalRange := meterstat.TimeRange{T1: future}
	for location, names := range p.Meters {
		trange := meterstat.TimeRange{T1: future}
		for _, name := range names {
			meterDir := filepath.Join(p.SampleDir, name)
			sd, err := meterstat.ReadSampleDir(meterDir, "*.sample")
			if err != nil {
				return nil, fmt.Errorf("cannot read sample dir %v: %v", meterDir, err)
			}
			meterDirs[location] = append(meterDirs[location], sd)
			trange = trange.Intersect(sd.Range)
		}
		locRange[location] = trange
		totalRange = totalRange.Intersect(trange)
	}
	// Find out what reports are possible.
	var reports []*Report
	for year := totalRange.T0.Year(); year <= totalRange.T1.Year(); year++ {
		for month := time.January; month <= time.December; month++ {
			// We can generate a report if we've got some samples
			// within the month for all meters.
			monthRange := meterstat.TimeRange{
				T0: time.Date(year, month, 1, 0, 0, 0, 0, p.TZ),
			}
			monthRange.T1 = monthRange.T0.AddDate(0, 1, 0)
			trange := monthRange
			for location := range meterDirs {
				// Note: if we only have less than an hour's worth of samples.
				// then we have to discard them because the report generation
				// requires full-hour multiples.
				trange = trange.Intersect(locRange[location].Constrain(time.Hour))
			}
			if trange.T1.After(trange.T0) {
				// There's a non-empty range of values, so it's a valid report.
				reports = append(reports, &Report{
					MeterDirs: meterDirs,
					Range:     trange,
					Partial:   !trange.Equal(monthRange),
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
	// Range holds the time range of the report.
	Range meterstat.TimeRange
	tz    *time.Location
	// Partial is true when the report doesn't cover the entire
	// expected period because of lack of available data.
	Partial bool
}

// Params returns the parameters for WriteReport.
func (r Report) Params() Params {
	locUsageReaders := make(map[MeterLocation]meterstat.UsageReader)
	for loc, sds := range r.MeterDirs {
		usageReaders := make([]meterstat.UsageReader, 0, len(sds))
		for _, sd := range sds {
			usageReaders = append(usageReaders, meterstat.NewUsageReader(sd.OpenRange(r.Range), r.Range.T0, time.Minute))
		}
		locUsageReaders[loc] = meterstat.SumUsage(usageReaders...)
	}
	return Params{
		Generator: locUsageReaders[LocGenerator],
		Neighbour: locUsageReaders[LocNeighbour],
		Here:      locUsageReaders[LocHere],
		EndTime:   r.Range.T1,
		TZ:        r.tz,
	}
}

// Write writes the report as a CSV to w.
func (r *Report) Write(w io.Writer) error {
	rr, err := Open(r.Params())
	if err != nil {
		return err
	}
	return Write(w, rr)
}

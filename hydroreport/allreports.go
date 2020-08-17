package hydroreport

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/rogpeppe/hydro/meterstat"
)

type MeterLocation int

const (
	LocUnknown MeterLocation = iota
	LocGenerator
	LocNeighbour
	LocHere
)

var future = time.Date(3000, time.January, 1, 0, 0, 0, 0, time.UTC)

// AllReports returns a report for each possible monthly report that can be
// made from the data in the given directory.
//
// The meter names map holds the set of meter names for each meter
// location (typically the this contains the host name of the meter).
// The data for that meter is assumed to be in $dir/$name.
//
// The tz parameter holds the time zone to use for the generated reports
// (UTC if it's nil)
//
// Invalid files will be ignored.
//
// A report can only be generated for a given month if there's sample data
// spanning that month for all specified meters.
func AllReports(dir string, meterNames map[MeterLocation][]string, tz *time.Location) ([]*Report, error) {
	if len(meterNames) != 3 {
		return nil, fmt.Errorf("missing meter names for some meter locations")
	}
	if tz == nil {
		tz = time.UTC
	}
	// t0loc holds the latest start time of for each of a given location's sample directories.
	t0loc := make(map[MeterLocation]time.Time)
	// t0loc holds the earliest end time of for each of a given location's sample directories.
	t1loc := make(map[MeterLocation]time.Time)
	meterDirs := make(map[MeterLocation][]*meterSampleDir)
	for location, names := range meterNames {
		for _, name := range names {
			meterDir := filepath.Join(dir, name)
			sd, err := readMeterDir(meterDir)
			if err != nil {
				return nil, fmt.Errorf("cannot read meter dir %v: %v", meterDir, err)
			}
			meterDirs[location] = append(meterDirs[location], sd)
			t0, ok := t0loc[location]
			if !ok || sd.t0.After(t0) {
				t0loc[location] = sd.t0
			}
			t1, ok := t1loc[location]
			if !ok || sd.t1.Before(t1) {
				t1loc[location] = sd.t1
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
			t0r := time.Date(year, month, 1, 0, 0, 0, 0, tz)
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
					meterDirs: meterDirs,
					t0:        t0r,
					t1:        t1r,
					tz:        tz,
				})
			}
		}
	}
	return reports, nil
}

var errNoSamples = fmt.Errorf("no samples found")

func readMeterDir(dir string) (*meterSampleDir, error) {
	infos, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []sampleFile
	t0 := time.Now()
	var t1 time.Time
	for _, info := range infos {
		if (info.Mode() & os.ModeType) != 0 {
			continue
		}
		path := filepath.Join(dir, info.Name())
		t0f, t1f, err := meterstat.SampleFileTimeRange(path)
		if err != nil {
			continue
		}
		files = append(files, sampleFile{
			path: path,
			t0:   t0f,
			t1:   t1f,
		})
		if t0f.Before(t0) {
			t0 = t0f
		}
		if t1f.After(t1) {
			t1 = t1f
		}
	}
	if t1.IsZero() {
		// No valid files found.
		return nil, errNoSamples
	}
	return &meterSampleDir{
		files: files,
		t0:    t0,
		t1:    t1,
	}, nil
}

type meterSampleDir struct {
	files  []sampleFile
	t0, t1 time.Time
}

type sampleFile struct {
	path   string
	t0, t1 time.Time
}

type Report struct {
	meterDirs map[MeterLocation][]*meterSampleDir
	// t0 and t1 hold the time range of the report.
	t0, t1 time.Time
	tz     *time.Location
}

func (r *Report) StartTime() time.Time {
	return r.t0
}

func (r *Report) Write(w io.Writer) error {
	locUsageReaders := make(map[MeterLocation]meterstat.UsageReader)
	for loc, sds := range r.meterDirs {
		usageReaders := make([]meterstat.UsageReader, 0, len(sds))
		for _, sd := range sds {
			sampleReaders := make([]meterstat.SampleReader, 0, len(sd.files))
			for _, file := range relevantFiles(sd.files, r.t0, r.t1) {
				f, err := meterstat.OpenSampleFile(file.path)
				if err != nil {
					return err
				}
				defer f.Close()
				sampleReaders = append(sampleReaders, f)
			}
			if len(sampleReaders) == 0 {
				// Shouldn't happen because there should always be at least one sample file.
				panic("no sample readers added")
			}
			allSamples := meterstat.MultiSampleReader(sampleReaders...)
			usageReaders = append(usageReaders, meterstat.NewUsageReader(allSamples, r.t0, time.Minute))
		}
		locUsageReaders[loc] = meterstat.SumUsage(usageReaders...)
	}
	return WriteReport(w, ReportParams{
		Generator: locUsageReaders[LocGenerator],
		Neighbour: locUsageReaders[LocNeighbour],
		Here:      locUsageReaders[LocHere],
		EndTime:   r.t1,
		TZ:        r.tz,
	})
}

// relevantFiles returns all of the given sample files that are useful
// for evaluating data points from t0 to t1.
// We only need to keep files that have data ranges that overlap
// the interval or that are directly before or after it if we
// don't yet have a file that overlaps [t0, t0] or [t1, t1] respectively.
func relevantFiles(sds []sampleFile, t0, t1 time.Time) []sampleFile {
	result := make([]sampleFile, 0, len(sds))
	haveStart := false
	haveEnd := false
	var start, end sampleFile
	for _, sd := range sds {
		if timeOverlaps(sd.t0, sd.t1, t0, t1) {
			result = append(result, sd)
			haveStart = haveStart || timeOverlaps(sd.t0, sd.t1, t0, t0)
			haveEnd = haveEnd || timeOverlaps(sd.t0, sd.t1, t1, t1)
			continue
		}
		if !haveStart && (start.path == "" || sd.t1.After(start.t1)) {
			start = sd
		}
		if !haveEnd && (end.path == "" || sd.t0.Before(end.t0)) {
			end = sd
		}
	}
	if !haveStart && start.path != "" {
		result = append(result, start)
	}
	if !haveEnd && end.path != "" {
		result = append(result, end)
	}
	return result
}

func timeOverlaps(at0, at1, bt0, bt1 time.Time) bool {
	if at1.Before(at0) {
		panic("bad interval a")
	}
	if bt1.Before(bt0) {
		panic("bad interval b")
	}
	return !at0.After(bt1) && !bt0.After(at1)
}

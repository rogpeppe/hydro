package hydroreport

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/meterstat"
)

var approxDeepEquals = qt.CmpEquals(cmpopts.EquateApprox(0, 0.001))

func TestAllReports(t *testing.T) {
	c := qt.New(t)

	dir := c.Mkdir()
	for path, samples := range sampleDirContents {
		path = filepath.Join(dir, path)
		err := os.MkdirAll(filepath.Dir(path), 0777)
		c.Assert(err, qt.IsNil)
		var buf bytes.Buffer
		_, err = meterstat.WriteSamples(&buf, meterstat.NewMemSampleReader(samples))
		c.Assert(err, qt.IsNil)
		err = ioutil.WriteFile(path, buf.Bytes(), 0666)
		c.Assert(err, qt.IsNil)
	}
	reports, err := AllReports(AllReportsParams{
		SampleDir: dir,
		Meters: map[MeterLocation][]string{
			LocGenerator: {"generator-a"},
			LocHere:      {"here-a"},
			LocNeighbour: {"neighbour-a"},
		},
	})
	c.Log("here range", epoch.Add(month), epoch.Add(4*month))
	c.Assert(err, qt.IsNil)
	var startTimes []time.Time
	for _, r := range reports {
		startTimes = append(startTimes, r.Range.T0)
	}
	c.Assert(startTimes, qt.DeepEquals, []time.Time{
		date(2000, 12, 1),
		date(2001, 1, 1),
	})
	assertUniformReport(c, reports[0], date(2000, 12, 1), date(2001, 1, 1), time.Hour, hydroctl.PowerChargeable{
		ExportGrid:      36000,
		ExportNeighbour: 10000,
		ExportHere:      4000,
	})
	assertUniformReport(c, reports[1], date(2001, 1, 1), date(2001, 2, 1), time.Hour, hydroctl.PowerChargeable{
		ExportGrid:      36000,
		ExportNeighbour: 10000,
		ExportHere:      4000,
	})
}

func assertUniformReport(c *qt.C, r *Report, t0, t1 time.Time, interval time.Duration, expect hydroctl.PowerChargeable) {
	var buf bytes.Buffer
	err := r.Write(&buf)
	c.Assert(err, qt.IsNil)
	csvr := csv.NewReader(bytes.NewReader(buf.Bytes()))
	csvr.FieldsPerRecord = 6
	csvr.ReuseRecord = true
	// Skip header field.
	_, err = csvr.Read()
	c.Assert(err, qt.IsNil)
	expectFields := []string{
		"date",
		fmt.Sprintf("%.3f", expect.ExportGrid/1000),
		fmt.Sprintf("%.3f", expect.ExportNeighbour/1000),
		fmt.Sprintf("%.3f", expect.ExportHere/1000),
		fmt.Sprintf("%.3f", expect.ImportNeighbour/1000),
		fmt.Sprintf("%.3f", expect.ImportHere/1000),
	}

	for t := t0.In(time.UTC); t.Before(t1); t = t.Add(interval) {
		fields, err := csvr.Read()
		if err == io.EOF {
			break
		}
		c.Assert(err, qt.IsNil)
		expectFields[0] = t.Format("2006-01-02 15:04 MST")
		c.Assert(fields, approxDeepEquals, expectFields)
	}
}

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

var sampleDirContents = map[string][]meterstat.Sample{
	"generator-a/1.sample": {{
		Time:        epoch,
		TotalEnergy: 1000,
	}},
	"generator-a/2.sample": {{
		// approx seven months at 50kW (encloses 2000-11 to 2001-05)
		Time:        epoch.Add(7 * month),
		TotalEnergy: 1000 + float64(7*month/time.Hour)*50000,
	}},
	"here-a/1.sample": {{
		Time:        epoch.Add(month),
		TotalEnergy: 30000,
	}, {
		// approx 4 months at 3kW (encloses 2000-12 to 2001-02)
		Time:        epoch.Add(4 * month),
		TotalEnergy: 30000 + float64(4*month/time.Hour)*3000,
	}},
	"neighbour-a/1.sample": {{
		Time:        epoch.Add(-25 * time.Hour),
		TotalEnergy: 0,
	}, {
		// approx 9 months and a day at 9kW (encoses 2000-10 to 2001-07)
		Time:        epoch.Add(9 * month),
		TotalEnergy: float64((9*month/time.Hour)+25) * 10000,
	}},
	"neighbour-b/1.sample": {{
		Time:        epoch,
		TotalEnergy: 0,
	}, {
		// approx 12 months at 1kW
		Time:        epoch.Add(12 * month),
		TotalEnergy: float64(12*month/time.Hour) * 1000,
	}},
}

const month = 31 * 24 * time.Hour

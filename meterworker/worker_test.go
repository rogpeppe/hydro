package meterworker

import (
	"fmt"
	"log"
	"path/filepath"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/kr/fs"

	"github.com/rogpeppe/hydro/hydroreport"
	"github.com/rogpeppe/hydro/logworker"
	"github.com/rogpeppe/hydro/meterstat"
	"github.com/rogpeppe/hydro/ndmetertest"
)

func TestWorker(t *testing.T) {
	c := qt.New(t)
	// 0: generator
	// 1: neighbour
	// 2: here
	locations := []hydroreport.MeterLocation{
		hydroreport.LocGenerator,
		hydroreport.LocNeighbour,
		hydroreport.LocHere,
	}
	meterServers := make([]*ndmetertest.Server, 3)
	for i := range meterServers {
		var err error
		meterServers[i], err = ndmetertest.NewServer("localhost:0")
		c.Assert(err, qt.IsNil)
		log.Printf("started meter server at %q", meterServers[i].Addr)
	}
	now := time.Now()
	earliest := time.Now().AddDate(0, -3, -15)
	storageDuration := now.Sub(earliest)
	totalTime := now.Sub(earliest)
	// Generator genering continuously at 50kW
	meterServers[0].AddSamples(mkSamples(earliest, now, 5*time.Minute, 0, totalTime.Hours()*50000))
	// Neighbour using 10kW continuously
	meterServers[1].AddSamples(mkSamples(earliest, now, 5*time.Minute, 0, totalTime.Hours()*10000))
	// Here using 60kW continuously
	meterServers[2].AddSamples(mkSamples(earliest, now, 5*time.Minute, 0, totalTime.Hours()*60000))

	tmpDir := c.Mkdir()
	sampleDir := filepath.Join(tmpDir, "samples")
	meterConfigPath := filepath.Join(tmpDir, "meterconfig.json")
	reportc := make(chan []*hydroreport.Report, 10)
	mw, err := New(Params{
		Updater: funcUpdater{
			updateAvailableReports: func(reports []*hydroreport.Report) {
				select {
				case reportc <- reports:
				default:
					panic("report channel is full!")
				}
			},
		},
		MeterConfigPath: meterConfigPath,
		SampleDirPath:   sampleDir,
		TZ:              time.UTC,
		NewSampleWorker: func(p SampleWorkerParams) (SampleWorker, error) {
			c.Logf("starting logworker for %v", p.MeterAddr)
			w, err := logworker.New(logworker.Params{
				SampleDir:       p.SampleDir,
				MeterAddr:       p.MeterAddr,
				TZ:              p.TZ,
				PollInterval:    time.Second,
				StorageDuration: storageDuration,
				Prefix:          "log-",
			})
			if err != nil {
				return nil, err
			}
			return w, nil
		},
		ReportPollInterval: time.Second,
	})
	c.Assert(err, qt.IsNil)
	defer mw.Close()
	meters := make([]Meter, len(meterServers))
	for i, srv := range meterServers {
		meters[i] = Meter{
			Name:       fmt.Sprintf("meter %d", i),
			Addr:       srv.Addr,
			Location:   locations[i],
			AllowedLag: time.Millisecond,
		}
	}
	err = mw.SetMeters(meters)
	c.Assert(err, qt.IsNil)

	timeout := time.After(5 * time.Second)
	var reports []*hydroreport.Report
loop:
	for {
		select {
		case reports = <-reportc:
			t.Logf("got %d reports", len(reports))
			if len(reports) != 0 {
				break loop
			}
		case <-timeout:
			c.Logf("contents of %v", tmpDir)
			for walker := fs.Walk(tmpDir); walker.Step(); {
				c.Logf("%6d %s", walker.Stat().Size(), walker.Path())
			}
			c.Logf("----")
			t.Fatal("timed out waiting for reports")
		}
	}
	if len(reports) != 2 && len(reports) != 3 {
		t.Fatalf("expected 2 or 3 reports; got %v", qt.Format(reports))
	}
}

type funcUpdater struct {
	updateMeterState       func(ms *MeterState)
	updateAvailableReports func(reports []*hydroreport.Report)
}

func (u funcUpdater) UpdateMeterState(ms *MeterState) {
	if u.updateMeterState != nil {
		u.updateMeterState(ms)
	}
}

func (u funcUpdater) UpdateAvailableReports(reports []*hydroreport.Report) {
	if u.updateAvailableReports != nil {
		u.updateAvailableReports(reports)
	}
}

func mkSamples(t0, t1 time.Time, interval time.Duration, e0, e1 float64) []meterstat.Sample {
	var samples []meterstat.Sample
	for t := t0; !t.After(t1); t = t.Add(interval) {
		samples = append(samples, meterstat.Sample{
			Time:        t,
			TotalEnergy: float64(t.Sub(t0))/float64(t1.Sub(t0))*(e1-e0) + e0,
		})
	}
	return samples
}

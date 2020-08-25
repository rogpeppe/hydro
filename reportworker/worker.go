package reportworker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/rogpeppe/hydro/hydroreport"
)

type Params struct {
	SampleDir string
	// Meters holds the names of the meter directories within SampleDir.
	Meters       map[hydroreport.MeterLocation][]string
	TZ           *time.Location
	PollInterval time.Duration
	// UpdateAvailableReports is called to update the currently available reports.
	// This should not block (specifically, calling Worker.Close will cause a deadlock).
	// It's OK for the function to take ownership of the slice.
	UpdateAvailableReports func([]*hydroreport.Report)
}

type Worker struct {
	p     Params
	ctx   context.Context
	close func()
	wg    sync.WaitGroup
}

func New(p Params) (*Worker, error) {
	if p.SampleDir == "" {
		return nil, fmt.Errorf("no sample directory provided")
	}
	if p.PollInterval == 0 {
		p.PollInterval = 4 * time.Hour
	}
	if p.UpdateAvailableReports == nil {
		return nil, fmt.Errorf("no UpdateAvailableReports callback provided")
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &Worker{
		ctx:   ctx,
		close: cancel,
		p:     p,
	}
	w.wg.Add(1)
	go w.run()
	return w, nil
}

func (w *Worker) run() {
	defer w.wg.Done()
	for {
		reports, err := hydroreport.AllReports(hydroreport.AllReportsParams{
			SampleDir: w.p.SampleDir,
			Meters:    w.p.Meters,
			TZ:        w.p.TZ,
		})
		if err != nil {
			log.Printf("cannot gather reports: %v", err)
		}
		w.p.UpdateAvailableReports(reports)
		select {
		case <-w.ctx.Done():
			return
		case <-time.After(w.p.PollInterval):
		}
	}
}

func (w *Worker) Close() {
	w.close()
	w.wg.Wait()
}

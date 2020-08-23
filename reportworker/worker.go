package reportworker

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/rogpeppe/hydro/hydroreport"
)

type Params struct {
	SampleDir    string
	// Meters holds the names of the meter directories within SampleDir.
	Meters       map[hydroreport.MeterLocation][]string
	TZ           *time.Location
	PollInterval time.Duration
	// SetAvailableReports is called to set the currently available reports.
	// This should not block (specifically, calling Worker.Close will cause a deadlock).
	// It's OK for the function to take ownership of the slice.
	SetAvailableReports func([]*hydroreport.Report)
}

type Worker struct {
	p        Params
	closed   chan struct{}
	isClosed bool
	wg       sync.WaitGroup
}

func New(p Params) (*Worker, error) {
	if p.SampleDir == "" {
		return nil, fmt.Errorf("no sample directory provided")
	}
	if p.PollInterval == 0 {
		p.PollInterval = 4 * time.Hour
	}
	if p.SetAvailableReports == nil {
		return nil, fmt.Errorf("no SetAvailableReports callback provided")
	}
	w := &Worker{
		p:      p,
		closed: make(chan struct{}),
	}
	w.wg.Add(1)
	go w.run()
	return w, nil
}

func (w *Worker) run() {
	for {
		reports, err := hydroreport.AllReports(hydroreport.AllReportsParams{
			SampleDir: w.p.SampleDir,
			Meters:    w.p.Meters,
			TZ:        w.p.TZ,
		})
		if err != nil {
			log.Printf("cannot gather reports: %v", err)
		}
		w.p.SetAvailableReports(reports)
		select {
		case <-w.closed:
			return
		case <-time.After(w.p.PollInterval):
		}
	}
}

func (w *Worker) Close() {
	if w.isClosed {
		return
	}
	w.isClosed = true
	close(w.closed)
	w.wg.Wait()
}

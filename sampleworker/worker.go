// Package sampleworker provides a worker that polls a meter and stores its ongoing total energy readings.
//
// It produces at least one file per time it's started, but also produces a new file every day.
package sampleworker

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rogpeppe/hydro/ndmeter"
	"gopkg.in/retry.v1"
)

type Params struct {
	// SampleDir holds the name of the directory to store the files.
	SampleDir string
	// MeterAddr holds the address of the meter.
	MeterAddr string
	// Prefix is used as a prefix for the file names created in SampleDir.
	Prefix string
	// Now is used to query the current time. If it's nil, time.Now will be used.
	Now func() time.Time
	// Interval holds the sampling interval.
	// If it's zero, DefaultInterval will be used.
	Interval time.Duration
}

const DefaultInterval = 30 * time.Second

// New returns a new Worker that polls a meter and stores energy readings
// files in the format understood by hydroreport.NewSampleReader.
func New(p Params) (*Worker, error) {
	if p.SampleDir == "" {
		return nil, fmt.Errorf("no sample directory set")
	}
	if p.MeterAddr == "" {
		return nil, fmt.Errorf("no meter address set")
	}
	if p.Now == nil {
		p.Now = time.Now
	}
	if p.Interval == 0 {
		p.Interval = DefaultInterval
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &Worker{
		p:     p,
		ctx:   ctx,
		close: cancel,
	}
	w.wg.Add(1)
	go func() {
		if err := w.run(); err != nil {
			log.Printf("sample worker for meter at %q failed: %v", w.p.MeterAddr, err)
		}
	}()
	return w, nil
}

type Worker struct {
	p        Params
	isClosed bool
	ctx      context.Context
	close    func()
	wg       sync.WaitGroup
}

func (w *Worker) Close() {
	w.close()
	w.wg.Wait()
}

func (w *Worker) run() error {
	defer w.wg.Done()
	var prevSampleTime time.Time
	var outf *os.File
	defer func() {
		if outf != nil {
			if err := outf.Close(); err != nil {
				log.Printf("failed to close sample file %q: %v", outf.Name(), err)
			}
		}
	}()

	for {
		totalEnergy, ok := w.readMeter()
		if !ok {
			return nil
		}
		now := w.p.Now()
		if !samePeriod(prevSampleTime, now) || outf == nil {
			if outf != nil {
				if err := outf.Close(); err != nil {
					log.Printf("failed to close sample file %q: %v", outf.Name(), err)
				}
				outf = nil
			}
			f, err := os.Create(w.filename(now))
			if err != nil {
				return err
			}
			outf = f
		}
		if _, err := fmt.Fprintf(outf, "%d,%g\n", now.UnixNano()/1e6, totalEnergy); err != nil {
			log.Printf("cannot write sample to %q: %v", outf.Name(), err)
		}
		select {
		case <-time.After(w.p.Interval):
		case <-w.ctx.Done():
			return nil
		}
	}
}

// timeFormat is the format we use for the time in the filenames.
// We omit colons so that it's compatible with windows filesystems.
const timeFormat = "2006-01-02T150405.000Z0700"

func (w *Worker) filename(startTime time.Time) string {
	return filepath.Join(w.p.SampleDir, w.p.Prefix+startTime.Format(timeFormat))
}

var retryStrategy = retry.Exponential{
	Initial:  100 * time.Millisecond,
	Factor:   1.5,
	MaxDelay: 5 * time.Second,
}

func (w *Worker) readMeter() (float64, bool) {
	for a := retry.StartWithCancel(retryStrategy, nil, w.ctx.Done()); a.Next(); {
		reading, err := ndmeter.Get(w.ctx, w.p.MeterAddr)
		if err == nil {
			return reading.TotalEnergy, true
		}
		log.Printf("cannot get reading from %v: %v", w.p.MeterAddr, err)
	}
	// Note: this only happens when the context gets cancelled (i.e. the worker is closed).
	return 0, false
}

// samePeriod reports whether the two times are from the same
// reporting period. We produce at most one reporting file
// per restart per day.
func samePeriod(t0, t1 time.Time) bool {
	if t0.Year() != t1.Year() {
		return false
	}
	if t0.YearDay() != t1.YearDay() {
		return false
	}
	return true
}

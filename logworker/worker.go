package logworker

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rogpeppe/hydro/meterstat"
	"github.com/rogpeppe/hydro/ndmeter"
)

var ndmeterOpenEnergyLog = func(host string, t0, t1 time.Time) (sampleReadCloser, error) {
	r, err := ndmeter.OpenEnergyLog(host, t0, t1)
	if err != nil {
		return nil, err
	}
	return r, nil
}

type sampleReadCloser interface {
	ReadSample() (meterstat.Sample, error)
	Close() error
}

type Params struct {
	MeterHost       string
	Dir             string
	Prefix          string
	StorageDuration time.Duration
	PollInterval    time.Duration
	TZ              *time.Location
}

type Worker struct {
	p        Params
	closed   chan struct{}
	isClosed bool
	wg       sync.WaitGroup
}

// New returns a Worker that periodically scans a directory
// for sample files starting with a given prefix, looking for gaps in the sample record.
// When it identifies a day-long gap, it tries to acquire the samples
// from the meter.
func New(p Params) (*Worker, error) {
	if p.PollInterval == 0 {
		p.PollInterval = 4 * time.Hour
	}
	if p.TZ == nil {
		p.TZ = time.UTC
	}
	if p.StorageDuration == 0 {
		p.StorageDuration = 28 * 24 * time.Hour
	}
	if p.Dir == "" {
		return nil, fmt.Errorf("empty sample directory name")
	}
	if p.MeterHost == "" {
		return nil, fmt.Errorf("empty meter hostname")
	}
	if err := os.MkdirAll(p.Dir, 0777); err != nil {
		return nil, fmt.Errorf("cannot create sample directory: %v", err)
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
	defer w.wg.Done()
	for {
		if err := w.poll(); err != nil {
			log.Printf("%v", err)
		}
		select {
		case <-time.After(w.p.PollInterval):
		case <-w.closed:
			return
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

func (w *Worker) Params() Params {
	return w.p
}

func (w *Worker) poll() error {
	// Find the earliest time that we might obtain a sample and round
	// it up to the nearest day.
	t0 := time.Now().In(w.p.TZ).Add(-w.p.StorageDuration)
	t0 = time.Date(t0.Year(), t0.Month(), t0.Day(), 0, 0, 0, 0, w.p.TZ)
	t0 = t0.AddDate(0, 0, 1)
	t1 := time.Now().Add(-time.Minute)
	var need []time.Time
	for t := t0; t.AddDate(0, 0, 1).Before(t1); t = t.AddDate(0, 0, 1) {
		if w.need(t) {
			need = append(need, t)
		}
	}
	if len(need) == 0 {
		log.Printf("no new samples needed")
		return nil
	}
	for _, t := range need {
		if err := w.downloadSamples(t); err != nil {
			log.Printf("cannot create sample file %q: %v", w.filename(t), err)
		}
	}
	return nil
}

func (w *Worker) downloadSamples(t time.Time) (err error) {
	r, err := ndmeterOpenEnergyLog(w.p.MeterHost, t, t.AddDate(0, 0, 1))
	if err != nil {
		return err
	}
	defer r.Close()
	log.Printf("fetching %v", w.filename(t))
	f, err := ioutil.TempFile(w.p.Dir, "")
	if err != nil {
		return fmt.Errorf("cannot create temp file: %v", err)
	}
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(f.Name())
		}
	}()
	if err := meterstat.WriteSamples(f, r); err != nil {
		return fmt.Errorf("cannot write samples: %v", err)
	}
	f.Close()
	if err := os.Rename(f.Name(), w.filename(t)); err != nil {
		return fmt.Errorf("cannot rename temp file: %v", err)
	}
	return nil
}

const leeway = time.Hour

func (w *Worker) need(t time.Time) bool {
	endPeriod := t.AddDate(0, 0, 1)
	path := w.filename(t)
	info, err := meterstat.SampleFileInfo(path)
	if err != nil {
		return true
	}
	t0, t1 := info.FirstSample().Time, info.LastSample().Time
	if t0.After(t.Add(leeway)) || t1.Before(endPeriod.Add(-leeway)) {
		log.Printf("samples out of range; range [%v %v] need [%v %v]", t0, t1, t.Add(leeway), endPeriod.Add(-leeway))
		// It doesn't contain all the samples we'd like it to
		return true
	}
	return false
}

func (w *Worker) filename(t time.Time) string {
	name := fmt.Sprintf("%s%s.sample", w.p.Prefix, t.Format("2006-01-02"))
	return filepath.Join(w.p.Dir, name)
}

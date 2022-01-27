package logworker

import (
	"context"
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

var ndmeterOpenEnergyLog = func(ctx context.Context, host string, t0, t1 time.Time) (sampleReadCloser, error) {
	r, err := ndmeter.OpenEnergyLog(ctx, host, t0, t1)
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
	// SampleDir holds the name of the directory to store the files.
	SampleDir string
	// MeterAddr holds the address of the meter.
	MeterAddr string
	// Prefix is used as a prefix for the file names created in SampleDir.
	Prefix string
	// StorageDuration holds the length of time that the meter holds the logs for.
	StorageDuration time.Duration
	// PollInterval holds how often to poll for new logs.
	PollInterval time.Duration
	// TZ holds the time zone to use when calculating day boundaries
	// to use for the sample names.
	TZ *time.Location
	// SamplesChanged is called if non-nil to notify that some new samples
	// have been added.
	SamplesChanged func()
}

type Worker struct {
	p     Params
	ctx   context.Context
	close func()
	wg    sync.WaitGroup
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
	if p.SampleDir == "" {
		return nil, fmt.Errorf("empty sample directory name")
	}
	if p.MeterAddr == "" {
		return nil, fmt.Errorf("empty meter address")
	}
	if err := os.MkdirAll(p.SampleDir, 0777); err != nil {
		return nil, fmt.Errorf("cannot create sample directory: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &Worker{
		p:     p,
		ctx:   ctx,
		close: cancel,
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
		case <-w.ctx.Done():
			return
		}
	}
}

func (w *Worker) Close() {
	w.close()
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
		n, err := w.downloadSamples(t)
		if err != nil {
			if w.ctx.Err() == nil {
				log.Printf("cannot create sample file %q: %v", w.filename(t), err)
			}
		} else {
			log.Printf("downloaded %d samples from %v starting at %v", n, w.p.MeterAddr, t)
			if w.p.SamplesChanged != nil {
				w.p.SamplesChanged()
			}
		}
	}
	return nil
}

func (w *Worker) downloadSamples(t time.Time) (n int, err error) {
	r, err := ndmeterOpenEnergyLog(w.ctx, w.p.MeterAddr, t, t.AddDate(0, 0, 1))
	if err != nil {
		return 0, err
	}
	defer r.Close()
	log.Printf("fetching %v", w.filename(t))
	f, err := ioutil.TempFile(w.p.SampleDir, "")
	if err != nil {
		return 0, fmt.Errorf("cannot create temp file: %v", err)
	}
	defer func() {
		f.Close()
		if err != nil {
			os.Remove(f.Name())
		}
	}()
	n, err = meterstat.WriteSamples(f, r)
	if err != nil {
		return 0, fmt.Errorf("cannot write samples: %v", err)
	}
	if err := f.Close(); err != nil {
		return 0, fmt.Errorf("cannot close output file: %v", err)
	}
	if n == 0 {
		return 0, fmt.Errorf("no samples found at %v", t)
	}
	if err := os.Rename(f.Name(), w.filename(t)); err != nil {
		return 0, fmt.Errorf("cannot rename temp file: %v", err)
	}
	return n, nil
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
	return filepath.Join(w.p.SampleDir, name)
}

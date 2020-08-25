// Package meterworker implements a worker that manages the current set of meters used
// by the system.
package meterworker

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/hydroreport"
	"github.com/rogpeppe/hydro/hydroworker"
	"github.com/rogpeppe/hydro/ndmeter"
	"github.com/rogpeppe/hydro/reportworker"
	"gopkg.in/ctxutil.v1"
	"gopkg.in/errgo.v1"
)

// Params holds the parameters for a call to New.
type Params struct {
	// Updater holds methods which are called when things change.
	Updater Updater

	// MeterConfigPath holds the path of the file where the meter configuration
	// is stored.
	MeterConfigPath string

	// SampleDirPath holds the path to the directory where the meter
	// samples will be stored (each meter has its own directory within
	// SampleDirPath)
	SampleDirPath string

	// TZ holds the time zone to work in.
	TZ *time.Location

	// NewSampleWorker is used to start new workers to gather samples.
	// (possible implementations are the logworker and sampleworker packages).
	NewSampleWorker func(SampleWorkerParams) (SampleWorker, error)

	// ReportPollInterval holds the interval at which to poll for new reports.
	// If it's zero, the default will be chosen by the reportworker package.
	ReportPollInterval time.Duration
}

// SampleWorkerParams holds the parameters for creating a new sample worker.
type SampleWorkerParams struct {
	// SampleDir holds the directory to store the samples in (in files
	// within the directory).
	SampleDir string
	// MeterAddr holds the host:port address of the meter.
	MeterAddr string
	// TZ holds the time zone to use.
	TZ *time.Location
}

// SampleWorker represents a started sample worker.
type SampleWorker interface {
	Close()
}

// Updater is used by the meterworker to notify external entities
// when something changes. Implementations should not block.
type Updater interface {
	// UpdateMeterState updates the current state of the meters
	// whenever it's known to have changed.
	// The method may retain a reference to ms but must not
	// mutate anything in it.
	UpdateMeterState(ms *MeterState)

	// UpdateAvailableReports updates the reports that can
	// currently be generated.
	UpdateAvailableReports(reports []*hydroreport.Report)
}

// MeterState represents a current state of all the meters.
type MeterState struct {
	// Meters holds all the meters in use.
	Meters []Meter

	// Time holds the time by which all the meter readings were
	// acquired (>= max of all the sample times).
	Time time.Time

	// Chargeable holds information about the current chargeable
	// power allocation.
	Chargeable hydroctl.PowerChargeable

	// Use holds information about the power currently being used.
	Use hydroctl.PowerUse

	// Samples holds all the most recent readings, indexed
	// by meter address.
	Samples map[string]*MeterSample
}

// MeterSample holds a sample taken from a meter.
type MeterSample struct {
	*ndmeter.Sample
	// AllowedLag holds the lag that was allowed for this sample.
	// This is included so that the front-end display can make a
	// better decision as to whether to display the lag time for a
	// sample or not.
	AllowedLag time.Duration
}

// Meter holds a meter that can be read to find out what the system is doing.
type Meter struct {
	Name       string
	Location   hydroreport.MeterLocation
	Addr       string // host:port
	AllowedLag time.Duration
}

var _ hydroworker.MeterReader = (*Worker)(nil)

type readMetersReq struct {
	ctx   context.Context
	reply chan readMetersReply
}

type readMetersReply struct {
	sample hydroctl.PowerUseSample
	err    error
}

type setMetersReq struct {
	meters []Meter
	reply  chan error
}

// Worker runs work related to the current set of meters.
type Worker struct {
	ctx         context.Context
	close       func()
	wg          sync.WaitGroup
	p           Params
	readMetersC chan readMetersReq
	setMetersC  chan setMetersReq

	// The fields below are owned by the run goroutine.

	// sampler holds the sampler used to obtain meter readings.
	sampler *ndmeter.Sampler

	// meters holds the meters as set by SetMeters.
	meters []Meter

	// meterState holds the most recently known meter state.
	// It's updated whenever ReadMeters is called.
	meterState *MeterState

	// reportWorker holds the worker responsible for polling the
	// sample directory (as populated by the sample workers)
	// to find available reports.
	reportWorker *reportworker.Worker

	// sampleWorkers holds the currently running sample workers,
	// keyed by meter address.
	sampleWorkers map[string]SampleWorker
}

// meterConfig defines the format used to persistently store
// the meter configuration.
type meterConfig struct {
	Meters []Meter
}

// New returns a new worker instance.
// It should be closed after use.
func New(p Params) (*Worker, error) {
	var mcfg meterConfig
	err := readJSONFile(p.MeterConfigPath, &mcfg)
	if err != nil && !os.IsNotExist(err) {
		return nil, errgo.Notef(err, "cannot read config from %q", p.MeterConfigPath)
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := &Worker{
		ctx:         ctx,
		close:       cancel,
		readMetersC: make(chan readMetersReq),
		setMetersC:  make(chan setMetersReq),

		sampler:       ndmeter.NewSampler(),
		sampleWorkers: make(map[string]SampleWorker),
		p:             p,
	}
	w.wg.Add(1)
	go w.run(mcfg.Meters)
	return w, nil
}

// Close closes the worker and shuts it down.
func (w *Worker) Close() {
	w.close()
	w.wg.Wait()
}

// ReadMeters implements hydroworker.MeterReader by reading the meters if
// there are any. If there are none it assumes that all currently active
// relays use their maximum power.
//
// If the context is cancelled, it returns immediately with the
// most recently obtainable readings.
func (w *Worker) ReadMeters(ctx context.Context) (hydroctl.PowerUseSample, error) {
	// Make a cancel context to avoid a persistent goroutine in ctxutil.Join if
	// neither context is cancelled.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx = ctxutil.Join(ctx, w.ctx)

	req := readMetersReq{
		ctx:   ctx,
		reply: make(chan readMetersReply, 1),
	}
	select {
	case w.readMetersC <- req:
		r := <-req.reply
		return r.sample, r.err
	case <-ctx.Done():
		return hydroctl.PowerUseSample{}, ctx.Err()
	}
}

// SetMeters sets the meters that are currently in use.
func (w *Worker) SetMeters(ms []Meter) error {
	req := setMetersReq{
		reply:  make(chan error, 1),
		meters: ms,
	}
	select {
	case w.setMetersC <- req:
		return <-req.reply
	case <-w.ctx.Done():
		return w.ctx.Err()
	}
}

func (w *Worker) run(meters []Meter) {
	defer w.wg.Done()
	defer w.stopWorkers()
	if _, err := w.setMeters(meters); err != nil {
		log.Printf("cannot set meters initially: %v", err)
	}
	w.p.Updater.UpdateMeterState(w.meterState)
	for {
		select {
		case req := <-w.setMetersC:
			metersChanged, err := w.setMeters(req.meters)
			req.reply <- err
			if metersChanged {
				w.p.Updater.UpdateMeterState(w.meterState)
			}
		case req := <-w.readMetersC:
			sample, meterStateChanged, err := w.readMeters(req.ctx)
			req.reply <- readMetersReply{
				sample: sample,
				err:    err,
			}
			if meterStateChanged {
				w.p.Updater.UpdateMeterState(w.meterState)
			}
		case <-w.ctx.Done():
			return
		}
	}
}

func (w *Worker) stopWorkers() {
	if w.reportWorker != nil {
		w.reportWorker.Close()
		w.reportWorker = nil
	}
	for addr, sw := range w.sampleWorkers {
		sw.Close()
		delete(w.sampleWorkers, addr)
	}
}

// readMeters is the internal version of ReadMeters, called from within the worker goroutine.
// It also reports whether the meter state might have changed.
//
// Note that the context is a combination of the context from the ReadMeters call and the
// context within the worker.
func (w *Worker) readMeters(ctx context.Context) (_ hydroctl.PowerUseSample, meterStateChanged bool, _ error) {
	if w.meters == nil {
		return hydroctl.PowerUseSample{}, false, hydroworker.ErrNoMeters
	}

	places := make([]ndmeter.SamplePlace, len(w.meters))
	for i, m := range w.meters {
		places[i] = ndmeter.SamplePlace{
			Addr:       m.Addr,
			AllowedLag: m.AllowedLag,
		}
	}
	var failed []string

	// Note that this might take some time and changing the meter addresses
	// will block until it's done, but that doesn't seem too unreasonable.
	samples := w.sampler.GetAll(ctx, places...)
	now := time.Now()
	samplesByAddr := make(map[string]*MeterSample)
	for i, sample := range samples {
		if sample != nil {
			samplesByAddr[places[i].Addr] = &MeterSample{
				Sample:     sample,
				AllowedLag: places[i].AllowedLag,
			}
		} else {
			failed = append(failed, places[i].Addr)
		}
	}

	var pu hydroctl.PowerUseSample
	for i, m := range w.meters {
		sample := samples[i]
		if sample == nil {
			continue
		}
		if pu.T0.IsZero() || sample.Time.Before(pu.T0) {
			pu.T0 = sample.Time
		}
		if pu.T1.IsZero() || sample.Time.After(pu.T1) {
			pu.T1 = sample.Time
		}
		switch m.Location {
		case hydroreport.LocGenerator:
			pu.Generated += sample.ActivePower
		case hydroreport.LocHere:
			pu.Here += sample.ActivePower
		case hydroreport.LocNeighbour:
			pu.Neighbour += sample.ActivePower
		default:
			log.Printf("unknown meter location %v", m.Location)
		}
	}
	pc := hydroctl.ChargeablePower(pu.PowerUse)
	w.meterState = &MeterState{
		Time:       now,
		Chargeable: pc,
		Use:        pu.PowerUse,
		Meters:     w.meters,
		Samples:    samplesByAddr,
	}
	if len(failed) > 0 {
		return hydroctl.PowerUseSample{}, true, errgo.Newf("failed to get meter readings from %v", failed)
	}
	return pu, true, nil
}

// setMeters is the internal version of SetMeters, called from within the worker.run goroutine.
// It reports whether the meter state was updated.
func (w *Worker) setMeters(meters []Meter) (bool, error) {
	if reflect.DeepEqual(meters, w.meters) {
		return false, nil
	}
	// Guard against races by making a copy of the meters slice.
	meters = append([]Meter(nil), meters...)

	// TODO write config atomically.
	if err := writeJSONFile(w.p.MeterConfigPath, meterConfig{meters}); err != nil {
		return false, err
	}
	w.meters = meters
	// TODO preserve some existing meter state.
	w.meterState = &MeterState{
		Meters: meters,
	}
	if w.p.SampleDirPath == "" {
		// No samples, no reports.
		return true, nil
	}
	if err := w.restartReportWorker(); err != nil {
		return true, fmt.Errorf("cannot restart report worker: %v", err)
	}
	if err := w.ensureSampleWorkers(); err != nil {
		return true, fmt.Errorf("cannot ensure sample workers: %v", err)
	}
	return true, nil
}

func (w *Worker) ensureSampleWorkers() error {
	meters := make(map[string]Meter)
	for _, m := range w.meters {
		meters[m.Addr] = m
	}
	// Stop any existing workers that aren't now included.
	for addr := range w.sampleWorkers {
		if _, ok := meters[addr]; !ok {
			w.Close()
			delete(w.sampleWorkers, addr)
		}
	}
	// Start any new workers required.
	for addr, m := range meters {
		if _, ok := w.sampleWorkers[addr]; ok {
			continue
		}
		sw, err := w.p.NewSampleWorker(SampleWorkerParams{
			SampleDir: filepath.Join(w.p.SampleDirPath, meterDirName(m)),
			MeterAddr: addr,
			TZ:        w.p.TZ,
		})
		if err != nil {
			return fmt.Errorf("cannot start sample worker for %q: %v", addr, err)
		}
		w.sampleWorkers[addr] = sw
	}
	return nil
}

func (w *Worker) restartReportWorker() error {
	if w.reportWorker != nil {
		w.reportWorker.Close()
		w.reportWorker = nil
	}
	meterMap := make(map[hydroreport.MeterLocation][]string)
	for _, m := range w.meters {
		meterMap[m.Location] = append(meterMap[m.Location], meterDirName(m))
	}
	// Start the report gatherer worker.
	reportWorker, err := reportworker.New(reportworker.Params{
		SampleDir:              w.p.SampleDirPath,
		Meters:                 meterMap,
		TZ:                     w.p.TZ,
		PollInterval:           w.p.ReportPollInterval,
		UpdateAvailableReports: w.p.Updater.UpdateAvailableReports,
	})
	if err != nil {
		return errgo.Notef(err, "cannot create report worker")
	}
	w.reportWorker = reportWorker
	return nil
}

func readJSONFile(path string, x interface{}) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, x)
}

func writeJSONFile(path string, x interface{}) error {
	data, err := json.Marshal(x)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, data, 0666)
}

// meterDirName returns the name for the sample directory for the given meter (within the sample directory).
func meterDirName(m Meter) string {
	// TODO ideally we'd use a more resilient name for the meter, such
	// as its mac address, but this'll do for now.
	return strings.ToLower(m.Location.String()) + "-" + strings.ReplaceAll(m.Addr, ":", "·")
}

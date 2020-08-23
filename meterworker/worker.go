// Package meterworker implements a worker that manages the current set of meters used
// by the system.
package meterworker

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/hydroreport"
	"github.com/rogpeppe/hydro/hydroworker"
	"github.com/rogpeppe/hydro/ndmeter"
	"gopkg.in/errgo.v1"
)

// Params holds the parameters for a call to New.
type Params struct {
	// Updater holds methods which are called when things change.
	Updater         Updater
	MeterConfigPath string
	SampleDirPath   string
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

// Worker runs work related to the current set of meters.
type Worker struct {
	//reportWorker *reportworker.Worker

	// sampler holds the sampler used to obtain meter readings.
	sampler *ndmeter.Sampler
	p       Params

	// mu guards the values below it. Note: we take care to avoid
	// changing the contents of the values because they're read concurrently.
	// Instead, we update the entire value when needed.
	mu sync.Mutex
	// meters holds the meters as set by SetMeters.
	meters []Meter
	// meterState holds the most recently known meter state.
	// It's updated whenever ReadMeters is called.
	meterState *MeterState
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
	//	if p.SampleDirPath != "" {
	//		// TODO Start one log-gatherer worker for each meter.
	//		//logw, err := logworker.New(
	//
	//		meters := make(map[hydroreport.MeterLocation][]string)
	//		mstate := store.meterState()
	//		log.Printf("got meter state from store: %#v", mstate)
	//		for _, m := range mstate.Meters {
	//			meters[m.Location] = append(meters[m.Location], meterName(m))
	//		}
	//		// Start the report gatherer worker.
	//		repw1, err := reportworker.New(reportworker.Params{
	//			SampleDir:           p.SampleDirPath,
	//			Meters:              meters,
	//			TZ:                  timezone,
	//			SetAvailableReports: store.SetAvailableReports,
	//		})
	//		if err != nil {
	//			return nil, errgo.Notef(err, "cannot create report worker")
	//		}
	//		repw = repw1
	//	}
	return &Worker{
		sampler: ndmeter.NewSampler(),
		meters:  mcfg.Meters,
		p:       p,
		meterState: &MeterState{
			Meters: mcfg.Meters,
		},
	}, nil
}

// Close closes the worker and shuts it down.
func (w *Worker) Close() {
	//if h.reportWorker != nil {
	//	h.reportWorker.Close()
	//}
}

// SetMeters sets the meters that are currently in use.
func (w *Worker) SetMeters(ms []Meter) error {
	meterState, err := w.setMeters(ms)
	if meterState != nil {
		w.p.Updater.UpdateMeterState(w.meterState)
	}
	return err
}

// setMeters is like SetMeters except that it doesn't call the updater
// if something changes. If the meter state was updated, it returns it.
func (w *Worker) setMeters(ms []Meter) (*MeterState, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if reflect.DeepEqual(ms, w.meters) {
		return nil, nil
	}
	// TODO write config atomically.
	if err := writeJSONFile(w.p.MeterConfigPath, meterConfig{ms}); err != nil {
		return nil, err
	}
	w.meters = ms
	// TODO preserve some existing meter state.
	w.meterState = &MeterState{
		Meters: ms,
	}
	return w.meterState, nil
}

// ReadMeters implements hydroworker.MeterReader by reading the meters if
// there are any. If there are none it assumes that all currently active
// relays use their maximum power.
//
// If the context is cancelled, it returns immediately with the
// most recently obtainable readings.
func (w *Worker) ReadMeters(ctx context.Context) (hydroctl.PowerUseSample, error) {
	sample, meterState, err := w.readMeters(ctx)
	if meterState != nil {
		w.p.Updater.UpdateMeterState(meterState)
	}
	if err != nil {
		return hydroctl.PowerUseSample{}, err
	}
	return sample, nil
}

// readMeters is like ReadMeters except that it doesn't call the updater.
// If the meter state has changed, it returns it.
func (w *Worker) readMeters(ctx context.Context) (hydroctl.PowerUseSample, *MeterState, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.meters == nil {
		return hydroctl.PowerUseSample{}, nil, hydroworker.ErrNoMeters
	}
	places := make([]ndmeter.SamplePlace, len(w.meters))
	for i, m := range w.meters {
		places[i] = ndmeter.SamplePlace{
			Addr:       m.Addr,
			AllowedLag: m.AllowedLag,
		}
	}
	var failed []string

	// Unlock so that we don't block everything
	// else while we're talking on the network.
	w.mu.Unlock()
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
	w.mu.Lock()

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
		return hydroctl.PowerUseSample{}, w.meterState, errgo.Newf("failed to get meter readings from %v", failed)
	}
	return pu, w.meterState, nil
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

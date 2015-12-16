package hydroworker

import (
	"log"
	"time"

	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/history"
	"gopkg.in/errgo.v1"
)

// TODO provide feedback of log messages to the front end

type NewParams struct {
	Config     *hydroctl.Config
	Store      history.Store
	Controller RelayController
	Meters     MeterReader
}

type Worker struct {
	controller RelayController
	meters     MeterReader
	history    *history.DB
	cfgChan    chan *hydroctl.Config
}

// RelayController represents an interface presented
// by a controller. It hides details such as connection
// drops (SetRelayers should retry) and relay state
// caching (Relays might not round-trip each time).
type RelayController interface {
	SetRelays(hydroctl.RelayState) error
	Relays() (hydroctl.RelayState, error)
}

// MeterReader represents a meter reader.
type MeterReader interface {
	// ReadMeters returns the most recent state of the
	// meters.
	ReadMeters() (hydroctl.MeterReading, error)
}

// Heartbeat is the interval at which the worker assesses for
// possible relay changes.
const Heartbeat = 500 * time.Millisecond

// New returns a new worker that updates the
func New(p NewParams) (*Worker, error) {
	hdb, err := history.New(p.Store)
	if err != nil {
		return nil, errgo.Mask(err)
	}
	w := &Worker{
		controller: p.Controller,
		meters:     p.Meters,
		history:    hdb,
		cfgChan:    make(chan *hydroctl.Config),
	}
	go w.run(p.Config)
	return w, nil
}

// SetConfig sets the current relay configuration.
// The caller must not mutate cfg after calling this function.
func (w *Worker) SetConfig(cfg *hydroctl.Config) {
	w.cfgChan <- cfg
}

// Close shuts down the worker.
func (w *Worker) Close() {
	close(w.cfgChan)
}

func (w *Worker) run(currentConfig *hydroctl.Config) {
	ticker := time.NewTicker(Heartbeat)
	defer ticker.Stop()
	for {
		currentRelays, err := w.controller.Relays()
		if err != nil {
			log.Printf("cannot get current relay state: %v", err)
			continue
		}
		currentMeters, err := w.meters.ReadMeters()
		if err != nil {
			log.Printf("cannot get current meter reading: %v", err)
			// What should we actually do here? Perhaps continuing would
			// be better.
			continue
		}
		now := time.Now()
		newRelays := hydroctl.Assess(currentConfig, currentRelays, w.history, currentMeters, now)
		if err := w.controller.SetRelays(newRelays); err != nil {
			log.Printf("cannot set relay state: %v", err)
			continue
		}
		if err := w.history.RecordState(newRelays, now); err != nil {
			log.Printf("cannot record state: %v", err)
		}
		select {
		case cfg, ok := <-w.cfgChan:
			if !ok {
				return
			}
			currentConfig = cfg
		case <-ticker.C:
		}
	}
}

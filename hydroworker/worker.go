// Package hydroworker implements the long-running worker that
// controls the electrics. It reads the meters and passes the configuration
// and meter readings to the hydroctl package, which will make the
// actual decisions.
package hydroworker

import (
	"log"
	"time"

	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/history"
	"github.com/rogpeppe/hydro/hydroctl"
)

// TODO provide feedback of log messages to the front end

// Params holds parameters for creating a new Worker.
type Params struct {
	// Config holds the initial relay configuration.
	Config *hydroctl.Config
	// Store is used to store events persistently.
	Store CommitStore
	// Controller is used to control the current relay state.
	Controller RelayController
	// Meters is used to read the meters.
	Meters MeterReader
	// Updater is used to inform external parties about the current state.
	// It may be nil.
	Updater Updater
}

// CommitStore adds a Commit method to the history.Store
// interface.
type CommitStore interface {
	history.Store

	// Commit adds the events queued by Append since
	// the last commit to the database.
	Commit() error
}

// Worker represents the worker goroutines.
type Worker struct {
	controller RelayController
	meters     MeterReader
	// history holds the history storage layer. It
	// uses Worker.store for its persistent state.
	history *history.DB

	store CommitStore

	updater Updater
	cfgChan chan *hydroctl.Config
}

// Updater is called when the current state changes.
// The call to UpdateWorkerState should not make
// any calls to the Worker - they might deadlock.
type Updater interface {
	UpdateWorkerState(u *Update)
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
const Heartbeat = 1000 * time.Millisecond

// New returns a new worker that keeps the relay state up to date
// with respect to configuration and meter changes.
func New(p Params) (*Worker, error) {
	hdb, err := history.New(p.Store)
	if err != nil {
		return nil, errgo.Mask(err)
	}
	w := &Worker{
		store:      p.Store,
		controller: p.Controller,
		meters:     p.Meters,
		history:    hdb,
		updater:    p.Updater,
		cfgChan:    make(chan *hydroctl.Config),
	}
	if w.updater == nil {
		w.updater = nopUpdater{}
	}
	go w.run(p.Config)
	return w, nil
}

type nopUpdater struct{}

func (nopUpdater) UpdateWorkerState(*Update) {
}

// SetConfig sets the current configuration.
// The caller must not mutate cfg after calling this function.
func (w *Worker) SetConfig(cfg *hydroctl.Config) {
	w.cfgChan <- cfg
}

// Close shuts down the worker.
func (w *Worker) Close() {
	close(w.cfgChan)
}

func (w *Worker) run(currentConfig *hydroctl.Config) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	firstTime := true
	var currentState Update
	for {
		select {
		case cfg, ok := <-w.cfgChan:
			if !ok {
				return
			}
			currentConfig = cfg
		case <-timer.C:
			timer.Reset(Heartbeat)
		}
		currentRelays, err := w.controller.Relays()
		if err != nil {
			log.Printf("cannot get current relay state: %v", err)
			continue
		}
		currentMeters, err := w.meters.ReadMeters()
		if err != nil {
			log.Printf("cannot get current meter reading: %v", err)
			// What should we actually do here? Is continuing the right choice?
			continue
		}
		now := time.Now()
		newRelays := hydroctl.Assess(currentConfig, currentRelays, w.history, currentMeters, now)
		changed := newRelays != currentRelays
		if changed {
			if err := w.controller.SetRelays(newRelays); err != nil {
				log.Printf("cannot set relay state: %v", err)
				continue
			}
		}
		if firstTime || changed {
			// The first time through the loop, even if the relay state might not
			// have changed from the actual state, the history might not
			// reflect the current state, so record it anyway.
			w.history.RecordState(newRelays, now)
			if err := w.store.Commit(); err != nil {
				log.Printf("cannot record state: %v", err)
			}
			w.updateState(&currentState, newRelays, firstTime)
			w.updater.UpdateWorkerState(currentState.Clone())
			firstTime = false
		}
	}
}

// updateState updates u to reflect the latest state stored in w.history,
// updating only those entries that have changed value,
// unless all is true, in which case all entries are updated.
func (w *Worker) updateState(u *Update, newState hydroctl.RelayState, all bool) {
	for i := range u.Relays {
		if !all && newState.IsSet(i) == u.State.IsSet(i) {
			continue
		}
		on, t := w.history.LatestChange(i)
		if on != newState.IsSet(i) {
			panic(errgo.Newf("unexpected result from history; relay %d expected %v got %v %v", i, newState.IsSet(i), on, t))
		}
		u.Relays[i] = RelayUpdate{
			On:    on,
			Since: t,
		}
	}
	u.State = newState
}

// Update holds information about the current worker state.
type Update struct {
	State  hydroctl.RelayState
	Relays [hydroctl.MaxRelayCount]RelayUpdate
}

// Clone returns a copy of *u.
func (u *Update) Clone() *Update {
	u1 := *u
	return &u1
}

// RelayUpdate holds information about the current state of a relay.
type RelayUpdate struct {
	On    bool
	Since time.Time
}

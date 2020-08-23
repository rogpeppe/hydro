package hydroserver

import (
	"io/ioutil"
	"os"
	"sync"

	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/hydroconfig"
	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/hydroreport"
	"github.com/rogpeppe/hydro/hydroworker"
	"github.com/rogpeppe/hydro/internal/notifier"
	"github.com/rogpeppe/hydro/meterworker"
)

type store struct {
	// configPath holds the file name where the configuration is stored.
	configPath string

	// configNotifier is updated when the configuration changes.
	configNotifier notifier.Notifier

	// anyNotifier is updated when any value (config, meters or worker state)
	// changes.
	anyNotifier notifier.Notifier

	// mu guards the values below it.
	mu sync.Mutex

	// configText holds the text of the configuration
	// as entered by the user.
	configText string

	// config holds the configuration that's derived
	// from configText.
	config *hydroconfig.Config

	// workerState holds the latest known worker state.
	workerState *hydroworker.Update

	// meterState_ holds the most recent meter state
	// as returned by ReadMeters.
	meterState_ *meterworker.MeterState

	// reports holds any currently available reports, as set with SetAvailableReports.
	reports []*hydroreport.Report
}

func newStore(configPath string) (*store, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, errgo.Mask(err)
	}
	cfg, err := hydroconfig.Parse(string(data))
	if err != nil {
		return nil, errgo.Mask(err)
	}

	return &store{
		configPath: configPath,
		config:     cfg,
		configText: string(data),
	}, nil
}

// ConfigText returns the current configuration string.
func (s *store) ConfigText() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.configText
}

// CtlConfig returns the current *hydroctl.Config value;
// the caller should not mutate the returned value.
func (s *store) CtlConfig() *hydroctl.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config.CtlConfig()
}

// Config returns the current relay configuration. The returned value
// must not be mutated.
func (s *store) Config() *hydroconfig.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

// setConfigText sets the relay configuration to the given string.
func (s *store) setConfigText(text string) error {
	cfg, err := hydroconfig.Parse(text)
	if err != nil {
		return errgo.Mask(err, errgo.Any)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if text == s.configText {
		return nil
	}
	// TODO write config atomically.
	// TODO should the store type be writing config files?
	if err := ioutil.WriteFile(s.configPath, []byte(text), 0666); err != nil {
		return errgo.Notef(err, "cannot write relay config file")
	}
	s.config = cfg
	s.configText = text
	// Notify any watchers.
	s.configNotifier.Changed()
	s.anyNotifier.Changed()
	return nil
}

// UpdateMeterState implements meterworker.Updater.UpdateMeterState.
func (s *store) UpdateMeterState(ms *meterworker.MeterState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meterState_ = ms
	s.anyNotifier.Changed()
}

// UpdateWorkerState sets the current worker state.
// It implements hydroworker.Updater.UpdaterWorkerState.
func (s *store) UpdateWorkerState(u *hydroworker.Update) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workerState = u
	// Notify any watchers.
	s.anyNotifier.Changed()
}

// WorkerState returns the current hydroworker state
// as set by SetWorkerState. The returned value must
// not be mutated.
func (s *store) WorkerState() *hydroworker.Update {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.workerState
}

// AvailableReports returns all the available reports. The caller
// should not mutate the return value.
func (s *store) AvailableReports() []*hydroreport.Report {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reports
}

// UpdateAvailableReports implements meterworker.Updater.UpdateAvailableReports.
func (s *store) UpdateAvailableReports(rs []*hydroreport.Report) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reports = rs
	s.anyNotifier.Changed()
}

// meterState returns the latest known meter state.
func (s *store) meterState() *meterworker.MeterState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.meterState_
}

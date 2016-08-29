package hydroserver

import (
	"io/ioutil"
	"os"
	"sync"

	"github.com/juju/utils/voyeur"
	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/hydroconfig"
	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/hydroworker"
)

type store struct {
	// path holds the file name where the configuration is stored.
	path string

	// configVal is update when the configuration changes.
	configVal voyeur.Value
	// anyVal is updated when any value (config, meters or worker state)
	// changes.
	anyVal      voyeur.Value
	mu          sync.Mutex
	config      *hydroconfig.Config
	workerState *hydroworker.Update
	meters      *hydroctl.MeterReading
	configText  string
}

func newStore(path string) (*store, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, errgo.Mask(err)
	}
	cfg, err := hydroconfig.Parse(string(data))
	if err != nil {
		return nil, errgo.Mask(err)
	}
	return &store{
		path:       path,
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

// SetConfigText sets the relay configuration to the given string.
func (s *store) SetConfigText(text string) error {
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
	if err := ioutil.WriteFile(s.path, []byte(text), 0666); err != nil {
		return errgo.Notef(err, "cannot write relay config file")
	}
	s.config = cfg
	s.configText = text
	// Notify any watchers.
	s.configVal.Set(nil)
	s.anyVal.Set(nil)
	return nil
}

// UpdateWorkerState sets the current worker state.
// It implements hydroworker.Updater.UpdaterWorkerState.
func (s *store) UpdateWorkerState(u *hydroworker.Update) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workerState = u
	// Notify any watchers.
	s.anyVal.Set(nil)
}

// WorkerState returns the current hydroworker state
// as set by SetWorkerState. The returned value must
// not be mutated.
func (s *store) WorkerState() *hydroworker.Update {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.workerState
}

// ReadMeters implements hydroworkd.MeterReader by assuming
// all currently active relays use their maximum power.
// TODO read actual meter readings or scrape off the web.
func (s *store) ReadMeters() (hydroctl.MeterReading, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.workerState == nil {
		return hydroctl.MeterReading{}, nil
	}
	total := 0
	for i := 0; i < hydroctl.MaxRelayCount; i++ {
		if s.workerState.State.IsSet(i) {
			total += s.config.Relays[i].MaxPower
		}
	}
	return hydroctl.MeterReading{
		Here:   total,
		Import: total,
	}, nil
}

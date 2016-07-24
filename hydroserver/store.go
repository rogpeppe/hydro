package hydroserver

import (
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
	meters      *hydroctl.MeterReading
	workerState *hydroworker.Update
	configText  string
}

func newStore() (*store, error) {
	return &store{
		config: &hydroconfig.Config{},
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

// Config returns the current relay configuration;
// the caller should not mutate the returned value.
func (s *store) Config() *hydroconfig.Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config
}

// SetConfigText sets the configuration to the given string.
func (s *store) SetConfigText(text string) error {
	cfg, err := hydroconfig.Parse(text)
	if err != nil {
		return errgo.Mask(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
	s.configText = text
	// Notify any watchers.
	s.configVal.Set(nil)
	s.anyVal.Set(nil)
	return nil
}

// UpdateWorkerState sets the current worker state.
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

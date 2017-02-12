package hydroserver

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"sync"

	"github.com/juju/utils/voyeur"
	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/hydroconfig"
	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/hydroworker"
	"github.com/rogpeppe/hydro/ndmeter"
)

type store struct {
	// path holds the file name where the configuration is stored.
	path string

	// sampler holds the sampler used to obtain meter readings.
	sampler *ndmeter.Sampler

	// configVal is updated when the configuration changes.
	configVal voyeur.Value
	// anyVal is updated when any value (config, meters or worker state)
	// changes.
	anyVal voyeur.Value

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

	// meterState holds the most recent meter state
	// as returned by ReadMeters.
	meterState *MeterState

	// meters holds the meters that will be read.
	meters []Meter
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

// SetMeters sets the meters to use for ReadMeters.
// The meters slice should not be changed after
// calling.
func (s *store) SetMeters(meters []Meter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meters = meters
	// TODO we could preserve some of the existing state.
	s.meterState = nil
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

// MeterState holds a meter state.
type MeterState struct {
	Chargeable hydroctl.PowerChargeable
	Use        hydroctl.PowerUse
	Meters     []Meter
	// Samples holds all the most readings, indexed
	// by meter address.
	Samples map[string]*ndmeter.Sample
}

// MeterState returns the latest known meter state.
func (s *store) MeterState() *MeterState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.meterState
}

// ReadMeters implements hydroworkd.MeterReader by reading the meters if
// there are any. If there are none it assumes that all currently active
// relays use their maximum power.
//
// If the context is cancelled, it returns immediately with the
// most recently obtainable readings.
func (s *store) ReadMeters(ctx context.Context) (hydroctl.PowerUseSample, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.meters == nil {
		return s.allMaxPower(), nil
	}
	addrs := make([]string, len(s.meters))
	for i, m := range s.meters {
		addrs[i] = m.Addr
	}
	var failed []string
	// Unlock so that we don't block everything
	// else while we're talking on the network.
	s.mu.Unlock()
	samples := s.sampler.GetAll(ctx, addrs...)
	samplesByAddr := make(map[string]*ndmeter.Sample)
	for i, sample := range samples {
		if sample != nil {
			samplesByAddr[addrs[i]] = sample
		} else {
			failed = append(failed, addrs[i])
		}
	}
	s.mu.Lock()
	var pu hydroctl.PowerUseSample
	for i, m := range s.meters {
		sample := samples[i]
		if pu.T0.IsZero() || sample.Time.Before(pu.T0) {
			pu.T0 = sample.Time
		}
		if pu.T1.IsZero() || sample.Time.After(pu.T1) {
			pu.T1 = sample.Time
		}
		switch m.Location {
		case LocGenerator:
			pu.Generated += sample.ActivePower
		case LocHere:
			pu.Here += sample.ActivePower
		case LocNeighbour:
			pu.Neighbour += sample.ActivePower
		default:
			log.Printf("unknown meter location %v", m.Location)
		}
	}
	pc := hydroctl.ChargeablePower(pu.PowerUse)
	s.meterState = &MeterState{
		Chargeable: pc,
		Use:        pu.PowerUse,
		Meters:     s.meters,
		Samples:    samplesByAddr,
	}
	if len(failed) > 0 {
		return hydroctl.PowerUseSample{}, errgo.Newf("failed to get meter readings from %v", failed)
	}
	return pu, nil
}

func (s *store) allMaxPower() hydroctl.PowerUseSample {
	if s.workerState == nil {
		return hydroctl.PowerUseSample{}
	}
	total := 0
	for i := 0; i < hydroctl.MaxRelayCount; i++ {
		if s.workerState.State.IsSet(i) {
			total += s.config.Relays[i].MaxPower
		}
	}
	return hydroctl.PowerUseSample{
		PowerUse: hydroctl.PowerUse{
			Here: float64(total),
		},
	}
}

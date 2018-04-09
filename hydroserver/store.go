package hydroserver

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/juju/utils/voyeur"
	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/hydroconfig"
	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/hydroworker"
	"github.com/rogpeppe/hydro/ndmeter"
)

type store struct {
	// configPath holds the file name where the configuration is stored.
	configPath string

	// metersPath holds the file name where the meter information is stored.
	metersPath string

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

	// meterState_ holds the most recent meter state
	// as returned by ReadMeters.
	meterState_ *meterState

	// meters holds the meters that will be read.
	meters []meter
}

func newStore(configPath, metersPath string) (*store, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, errgo.Mask(err)
	}
	cfg, err := hydroconfig.Parse(string(data))
	if err != nil {
		return nil, errgo.Mask(err)
	}

	var mcfg meterConfig
	err = readJSONFile(metersPath, &mcfg)
	if err != nil && !os.IsNotExist(err) {
		return nil, errgo.Notef(err, "cannot read config from %q", metersPath)
	}
	return &store{
		configPath:  configPath,
		metersPath:  metersPath,
		config:      cfg,
		configText:  string(data),
		sampler:     ndmeter.NewSampler(),
		meters:      mcfg.Meters,
		meterState_: new(meterState),
	}, nil
}

type meterLocation int

const (
	locUnknown meterLocation = iota
	locGenerator
	locNeighbour
	locHere
)

// meter holds a meter that can be read to find out what
// the system is doing.
type meter struct {
	Name       string
	Location   meterLocation
	Addr       string // host:port
	AllowedLag time.Duration
}

// meterConfig defines the format used to persistently store
// the meter configuration.
type meterConfig struct {
	Meters []meter
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
	if err := ioutil.WriteFile(s.configPath, []byte(text), 0666); err != nil {
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
func (s *store) setMeters(meters []meter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if reflect.DeepEqual(meters, s.meters) {
		return nil
	}
	// TODO write config atomically.
	if err := writeJSONFile(s.metersPath, meterConfig{meters}); err != nil {
		return errgo.Notef(err, "cannot write meters config file")
	}
	s.meters = meters
	// TODO we could preserve some of the existing state.
	s.meterState_ = &meterState{
		Meters: s.meters,
	}
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

// meterState holds a meter state.
type meterState struct {
	// Time holds the time by which all the meter readings were
	// acquired (>= max of all the sample times).
	Time       time.Time
	Chargeable hydroctl.PowerChargeable
	Use        hydroctl.PowerUse
	Meters     []meter
	// Samples holds all the readings, indexed
	// by meter address.
	Samples map[string]*meterSample
}

type meterSample struct {
	*ndmeter.Sample
	AllowedLag time.Duration
}

// meterState returns the latest known meter state.
func (s *store) meterState() *meterState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.meterState_
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
	places := make([]ndmeter.SamplePlace, len(s.meters))
	for i, m := range s.meters {
		places[i] = ndmeter.SamplePlace{
			Addr:       m.Addr,
			AllowedLag: m.AllowedLag,
		}
	}
	var failed []string
	// Unlock so that we don't block everything
	// else while we're talking on the network.
	s.mu.Unlock()
	samples := s.sampler.GetAll(ctx, places...)
	now := time.Now()
	samplesByAddr := make(map[string]*meterSample)
	for i, sample := range samples {
		if sample != nil {
			samplesByAddr[places[i].Addr] = &meterSample{
				Sample:     sample,
				AllowedLag: places[i].AllowedLag,
			}
		} else {
			failed = append(failed, places[i].Addr)
		}
	}
	s.mu.Lock()
	var pu hydroctl.PowerUseSample
	for i, m := range s.meters {
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
		case locGenerator:
			pu.Generated += sample.ActivePower
		case locHere:
			pu.Here += sample.ActivePower
		case locNeighbour:
			pu.Neighbour += sample.ActivePower
		default:
			log.Printf("unknown meter location %v", m.Location)
		}
	}
	pc := hydroctl.ChargeablePower(pu.PowerUse)
	s.meterState_ = &meterState{
		Time:       now,
		Chargeable: pc,
		Use:        pu.PowerUse,
		Meters:     s.meters,
		Samples:    samplesByAddr,
	}
	s.anyVal.Set(nil)
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

package hydroserver

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/eth8020"
	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/hydroworker"
)

type relayCtl struct {
	cfgStore *relayCtlConfigStore

	mu               sync.Mutex
	conn             *eth8020.Conn
	currentStateTime time.Time
	currentState     hydroctl.RelayState
}

// refreshDuration holds the maximum amount of time
// for which we will believe the most recently
// obtained relay settings.
const refreshDuration = 30 * time.Second

// TODO make the relay controller provide a notification when
// the relay state changes, so we can send the new relay
// state to any clients that are watching it.
//
// We also want to send the current meter readings.
// Do we want to have one websocket for both or a separate
// for each control scheme?
//
// Probably a single websocket with several different types of delta.

func newRelayController(cfgStore *relayCtlConfigStore) *relayCtl {
	return &relayCtl{
		cfgStore: cfgStore,
	}
}

func (ctl *relayCtl) SetRelayAddr(addr string) error {
	// TODO provide a way to change the password too.
	changed, err := ctl.cfgStore.SetRelayAddr(addr)
	if changed {
		ctl.mu.Lock()
		defer ctl.mu.Unlock()
		if ctl.conn != nil {
			ctl.conn.Close()
			ctl.conn = nil
		}
	}
	if err != nil {
		return errgo.Notef(err, "cannot set relay controller address")
	}
	return nil
}

func (ctl *relayCtl) RelayAddr() (string, error) {
	addr, err := ctl.cfgStore.RelayAddr()
	if err == nil || errgo.Cause(err) == hydroworker.ErrNoRelayController {
		return addr, nil
	}
	return "", errgo.Mask(err)
}

func (ctl *relayCtl) Relays() (hydroctl.RelayState, error) {
	ctl.mu.Lock()
	defer ctl.mu.Unlock()
	if !ctl.currentStateTime.IsZero() && time.Since(ctl.currentStateTime) < refreshDuration {
		return ctl.currentState, nil
	}
	var state eth8020.State
	err := ctl.retry(func() error {
		var err error
		state, err = ctl.conn.GetOutputs()
		return err
	})
	if err != nil {
		return 0, errgo.NoteMask(err, "cannot get current state", errgo.Is(hydroworker.ErrNoRelayController))
	}
	ctl.currentState = hydroctl.RelayState(state)
	ctl.currentStateTime = time.Now()
	return ctl.currentState, nil
}

// SetRelays implements hydroworker.RelayController.SetRelays.
func (ctl *relayCtl) SetRelays(state hydroctl.RelayState) error {
	ctl.mu.Lock()
	defer ctl.mu.Unlock()
	if err := ctl.retry(func() error {
		return ctl.conn.SetOutputs(eth8020.State(state))
	}); err != nil {
		return errgo.Notef(err, "cannot set relay state")
	}
	ctl.currentState = state
	ctl.currentStateTime = time.Now()
	return nil
}

// retry retries the given function (once) when the connection
// goes down. The function should not have any side effects
// on ctl, as at some point we'll add a timeout and side effects
// could lead to a race.
func (ctl *relayCtl) retry(f func() error) error {
	if err := ctl.connect(); err != nil {
		return errgo.Mask(err, errgo.Is(hydroworker.ErrNoRelayController))
	}
	err := f()
	if err == nil {
		return nil
	}
	log.Printf("relay controller: retrying after error: %v", err)
	// Retry, assuming the problem is because the
	// TCP connection has broken.
	ctl.conn.Close()
	ctl.conn = nil
	if err := ctl.connect(); err != nil {
		return errgo.Notef(err, "(on retry)")
	}
	if err := f(); err != nil {
		return errgo.Mask(err, errgo.Any)
	}
	return nil
}

func (ctl *relayCtl) connect() error {
	addr, err := ctl.cfgStore.RelayAddr()
	if err != nil {
		return errgo.Mask(err, errgo.Is(hydroworker.ErrNoRelayController))
	}
	if ctl.conn != nil {
		return nil
	}
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return errgo.Notef(err, "cannot connect to eth8020 controller")
	}
	econn := eth8020.NewConn(conn)
	state, err := econn.GetOutputs()
	if err != nil {
		econn.Close()
		return errgo.Notef(err, "cannot get current state (initially)")
	}
	ctl.conn = econn
	ctl.currentState = hydroctl.RelayState(state)
	ctl.currentStateTime = time.Now()
	return nil
}

// relayCtlConfigStore stores information on how to connect to
// the relay controller.
type relayCtlConfigStore struct {
	// path holds the filename that stores the address.
	path string

	mu  sync.Mutex
	cfg relayCtlConfig
}

type relayCtlConfig struct {
	Addr string
	// TODO add password too.
}

// SetRelayAddr sets the relay controller address.
// It reports whether the address has changed.
func (s *relayCtlConfigStore) SetRelayAddr(addr string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if addr == s.cfg.Addr {
		return false, nil
	}
	s.cfg.Addr = addr
	data, err := json.Marshal(s.cfg)
	if err != nil {
		return true, errgo.Mask(err)
	}
	if err := ioutil.WriteFile(s.path, data, 0666); err != nil {
		return true, errgo.Mask(err)
	}
	return true, nil
}

func (s *relayCtlConfigStore) RelayAddr() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := readJSONFile(s.path, &s.cfg); err != nil {
		if os.IsNotExist(err) {
			return "", hydroworker.ErrNoRelayController
		}
		return "", errgo.Notef(err, "badly formatted relay config data")
	}
	return s.cfg.Addr, nil
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

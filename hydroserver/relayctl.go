package hydroserver

import (
	"log"
	"net"
	"time"

	"gopkg.in/errgo.v1"

	"github.com/juju/utils/voyeur"
	"github.com/rogpeppe/hydro/eth8020"
	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/hydroworker"
)

type relayCtl struct {
	addr             string
	password         string
	conn             *eth8020.Conn
	currentStateTime time.Time
	currentState     hydroctl.RelayState
	val              voyeur.Value
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

func newRelayController(addr, password string) hydroworker.RelayController {
	return &relayCtl{
		addr:     addr,
		password: password,
	}
}

func (ctl *relayCtl) Relays() (hydroctl.RelayState, error) {
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
		return 0, errgo.Notef(err, "cannot get current state")
	}
	ctl.currentState = hydroctl.RelayState(state)
	ctl.currentStateTime = time.Now()
	return ctl.currentState, nil
}

func (ctl *relayCtl) SetRelays(state hydroctl.RelayState) error {
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
		return errgo.Mask(err)
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
	if ctl.conn != nil {
		return nil
	}
	conn, err := net.Dial("tcp", ctl.addr)
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

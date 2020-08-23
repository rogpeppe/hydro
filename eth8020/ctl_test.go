package eth8020_test

import (
	"net"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/rogpeppe/hydro/eth8020"
	"github.com/rogpeppe/hydro/eth8020test"
)

func TestGetSetOutputs(t *testing.T) {
	c := qt.New(t)
	srv, err := eth8020test.NewServer("localhost:0")
	c.Assert(err, qt.IsNil)
	defer srv.Close()
	netc, err := net.Dial("tcp", srv.Addr)
	c.Assert(err, qt.IsNil)
	conn := eth8020.NewConn(netc)
	defer conn.Close()
	state, err := conn.GetOutputs()
	c.Assert(err, qt.IsNil)
	c.Assert(state, qt.Equals, eth8020.State(0))

	err = conn.SetOutputs(0xcaa55)
	c.Assert(err, qt.IsNil)
	c.Assert(srv.State(), qt.Equals, eth8020.State(0xcaa55))

	state, err = conn.GetOutputs()
	c.Assert(err, qt.IsNil)
	c.Assert(state, qt.Equals, eth8020.State(0xcaa55))
}

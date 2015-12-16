package eth8020_test

import (
	"github.com/rogpeppe/hydro/eth8020"
	"github.com/rogpeppe/hydro/eth8020test"
	gc "gopkg.in/check.v1"
	"net"
)

type suite struct{}

var _ = gc.Suite(suite{})

func (suite) TestGetSetOutputs(c *gc.C) {
	srv := eth8020test.NewServer()
	defer srv.Close()
	netc, err := net.Dial("tcp", srv.Addr)
	c.Assert(err, gc.IsNil)
	conn := eth8020.NewConn(netc)
	defer conn.Close()
	state, err := conn.GetOutputs()
	c.Assert(err, gc.IsNil)
	c.Assert(state, gc.Equals, eth8020.State(0))

	err = conn.SetOutputs(0xcaa55)
	c.Assert(err, gc.IsNil)
	c.Assert(srv.State(), gc.Equals, eth8020.State(0xcaa55))

	state, err = conn.GetOutputs()
	c.Assert(err, gc.IsNil)
	c.Assert(state, gc.Equals, eth8020.State(0xcaa55))
}

package googlecharts_test

import (
	"encoding/json"
	"time"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/rogpeppe/hydro/googlecharts"
)

type tableSuite struct{}

var _ = gc.Suite(&tableSuite{})

type entry struct {
	Name  string
	X     int
	Y     float64 `googlecharts:"y label"`
	T     time.Time
	unexp int
}

func (*tableSuite) TestNewDataTable(c *gc.C) {
	dt := googlecharts.NewDataTable([]entry{{
		Name: "hello",
		X:    5,
		Y:    7,
		T:    time.Unix(1487509695, 123*1e6),
	}})
	data, err := json.Marshal(dt)
	c.Assert(err, gc.IsNil)
	c.Assert(string(data), jc.JSONEquals, &googlecharts.DataTable{
		Cols: []googlecharts.Column{{
			Type: "string",
			Id:   "Name",
		}, {
			Type: "number",
			Id:   "X",
		}, {
			Type:  "number",
			Id:    "Y",
			Label: "y label",
		}, {
			Type: "datetime",
			Id:   "T",
		}},
		Rows: []googlecharts.Row{{
			Cells: []googlecharts.Cell{{
				Value: "hello",
			}, {
				Value: 5,
			}, {
				Value: 7.0,
			}, {
				Value: "Date(1487509695123)",
			}},
		}},
	})
}

func (*tableSuite) TestNewDataTableWithPointerElements(c *gc.C) {
	dt := googlecharts.NewDataTable([]*entry{
		1: {
			Name: "hello",
			X:    5,
			Y:    7,
			T:    time.Unix(1487509695, 123*1e6),
		},
	})

	data, err := json.Marshal(dt)
	c.Assert(err, gc.IsNil)
	c.Assert(string(data), jc.JSONEquals, &googlecharts.DataTable{
		Cols: []googlecharts.Column{{
			Type: "string",
			Id:   "Name",
		}, {
			Type: "number",
			Id:   "X",
		}, {
			Type:  "number",
			Id:    "Y",
			Label: "y label",
		}, {
			Type: "datetime",
			Id:   "T",
		}},
		Rows: []googlecharts.Row{{
			Cells: make([]googlecharts.Cell, 4),
		}, {
			Cells: []googlecharts.Cell{{
				Value: "hello",
			}, {
				Value: 5,
			}, {
				Value: 7.0,
			}, {
				Value: "Date(1487509695123)",
			}},
		}},
	})
}

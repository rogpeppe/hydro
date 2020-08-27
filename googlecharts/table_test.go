package googlecharts_test

import (
	"encoding/json"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/rogpeppe/hydro/googlecharts"
)

type entry struct {
	Name  string
	X     int
	Y     float64 `googlecharts:"y label"`
	T     time.Time
	unexp int
}

func TestNewDataTable(t *testing.T) {
	c := qt.New(t)
	dt := googlecharts.NewDataTable([]entry{{
		Name: "hello",
		X:    5,
		Y:    7,
		T:    time.Unix(1487509695, 123*1e6),
	}})
	data, err := json.Marshal(dt)
	c.Assert(err, qt.IsNil)
	c.Assert(string(data), qt.JSONEquals, &googlecharts.DataTable{
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

func TestNewDataTableWithPointerElements(t *testing.T) {
	c := qt.New(t)
	dt := googlecharts.NewDataTable([]*entry{
		1: {
			Name: "hello",
			X:    5,
			Y:    7,
			T:    time.Unix(1487509695, 123*1e6),
		},
	})

	data, err := json.Marshal(dt)
	c.Assert(err, qt.IsNil)
	c.Assert(string(data), qt.JSONEquals, &googlecharts.DataTable{
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

type withEmbed struct {
	A int
	embed
}

type embed struct {
	B int
}

func TestNewDataTableWithAnonField(t *testing.T) {
	c := qt.New(t)
	dt := googlecharts.NewDataTable([]withEmbed{{
		A: 99,
		embed: embed{
			B: 88,
		},
	}})

	data, err := json.Marshal(dt)
	c.Assert(err, qt.IsNil)
	c.Assert(string(data), qt.JSONEquals, &googlecharts.DataTable{
		Cols: []googlecharts.Column{{
			Type: "number",
			Id:   "A",
		}, {
			Type: "number",
			Id:   "B",
		}},
		Rows: []googlecharts.Row{{
			Cells: []googlecharts.Cell{{
				Value: 99,
			}, {
				Value: 88,
			}},
		}},
	})
}

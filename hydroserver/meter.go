package hydroserver

import (
	"github.com/rogpeppe/hydro/hydroctl"
)

type meterReader struct {
}

func (meterReader) ReadMeters() (hydroctl.MeterReading, error) {
	// TODO read actual meter readings or scrape off the web.
	return hydroctl.MeterReading{}, nil
}

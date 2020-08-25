package ndmeter

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rogpeppe/hydro/meterstat"
)

const timeOffset = 315532800

// OpenEnergyLog opens a log of energy readings from the meter at the
// given host, requesting readings between t0 and t1.
// Note that the meter software is buggy, so the actually returned readings
// might not reflect the requested time range.
// The returned value should be closed after use.
func OpenEnergyLog(host string, t0, t1 time.Time) (*EnergyReader, error) {
	resp, err := http.PostForm("http://"+host+"/Read_Energy.cgi", url.Values{
		"From": {timeParam(t0)},
		"To":   {timeParam(t1)},
		"Fmt":  {"csv"},
	})
	if err != nil {
		return nil, fmt.Errorf("meter request failed: %v", err)
	}
	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() {
		return nil, fmt.Errorf("cannot read CSV header")
	}
	fields := csvFields(scanner.Text())
	if len(fields) < 3 || fields[0] != "Date" || fields[1] != "Time" || fields[2] != "kWh" {
		return nil, fmt.Errorf("CSV header does not have expected fields (%q)", scanner.Text())
	}
	return &EnergyReader{
		scanner: scanner,
		rc:      resp.Body,
	}, nil
}

type EnergyReader struct {
	scanner *bufio.Scanner
	rc      io.ReadCloser
}

// ReadSample implements meterstat.SampleReader.ReadSample
func (r *EnergyReader) ReadSample() (meterstat.Sample, error) {
	if !r.scanner.Scan() {
		err := r.scanner.Err()
		if err == nil {
			return meterstat.Sample{}, io.EOF
		}
		return meterstat.Sample{}, err
	}
	line := r.scanner.Text()
	if len(line) == 0 || line[0] == '<' {
		// The buggy software sends back a blank line and a spurious HTTP end tag
		// at the end, so pretend that's the real end.
		return meterstat.Sample{}, io.EOF
	}
	fields := csvFields(line)
	if len(fields) < 3 {
		return meterstat.Sample{}, fmt.Errorf("too few fields on CSV line %q", line)
	}
	timestamp, err := time.Parse("02-01-2006 15:04:05", fields[0]+" "+fields[1])
	if err != nil {
		return meterstat.Sample{}, fmt.Errorf("invalid timestamp in line %q", line)
	}
	energy, err := strconv.ParseFloat(fields[2], 64)
	if err != nil {
		return meterstat.Sample{}, fmt.Errorf("invalid energy reading in line %q", line)
	}
	return meterstat.Sample{
		Time:        timestamp,
		TotalEnergy: energy * 1000,
	}, nil
}

func (r *EnergyReader) Close() error {
	return r.rc.Close()
}

func csvFields(s string) []string {
	fields := strings.Split(s, ",")
	for i, f := range fields {
		fields[i] = strings.TrimSpace(f)
	}
	return fields
}

func timeParam(t time.Time) string {
	return fmt.Sprint(t.Unix() - timeOffset)
}

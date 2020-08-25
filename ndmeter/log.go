package ndmeter

import (
	"bufio"
	"context"
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

func postForm(ctx context.Context, url string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return http.DefaultClient.Do(req)
}

// OpenEnergyLog opens a log of energy readings from the meter at the
// given host, requesting readings between t0 and t1.
// Note that the meter software is buggy, so the actually returned readings
// might not reflect the requested time range.
// The returned value should be closed after use.
func OpenEnergyLog(ctx context.Context, host string, t0, t1 time.Time) (*EnergyReader, error) {
	resp, err := postForm(ctx, "http://"+host+"/Read_Energy.cgi", url.Values{
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
		first:   true,
		t0:      t0,
		t1:      t1,
	}, nil
}

type EnergyReader struct {
	scanner *bufio.Scanner
	t0, t1  time.Time
	rc      io.ReadCloser
	first   bool
}

// ReadSample implements meterstat.SampleReader.ReadSample.
func (r *EnergyReader) ReadSample() (meterstat.Sample, error) {
	// This buggy meter has a tendency to return samples outside of the requested
	// time range, so make sure we keep 'em in bounds.
	for {
		sample, err := r.readSample()
		if err != nil {
			return meterstat.Sample{}, err
		}
		if sample.Time.After(r.t1) {
			if r.first {
				return meterstat.Sample{}, fmt.Errorf("energy reading samples started out of bounds (got %v want between %v and %v)", sample.Time, r.t0, r.t1)
			}
			return meterstat.Sample{}, io.EOF
		}
		if !sample.Time.Before(r.t0) {
			r.first = false
			return sample, nil
		}
	}
}

// readSample is like ReadSample except that it doesn't check that
// the sample is within the requested bounds.
func (r *EnergyReader) readSample() (meterstat.Sample, error) {
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

// csvFields returns the fields from a CSV header as returned by the meter.
// Example:
// 	Date, Time, kWh, Export kWh, Counter 1, Counter 2, Counter 3
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

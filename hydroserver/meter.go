package hydroserver

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rogpeppe/hydro/meterstat"
	"github.com/rogpeppe/hydro/meterworker"
)

var meterTempl = newTemplate(`
<html>
<head>
<style>
.instructions {
	max-width: 30em
}
</style>
</head>
<body>
<h1>{{.Meter.Name}}</h1>
<form action="/samples/{{.Meter.Addr}}" method="POST">
<textarea name="samples" rows="20" cols="80">
{{range $s := .Samples}}{{$s.Time.Format "2006-01-02 15:04"}} {{printf "%.3fkWH" (mul $s.TotalEnergy .001)}}
{{end}}
</textarea><br>
<input type="submit" value="Save">
</body>
`)

type meterTemplParams struct {
	Meter   meterworker.Meter
	Samples []meterstat.Sample
}

func (h *Handler) serveMeters(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/meters/")
	if path == "" {
		// TODO could serve summary of meters.
		http.NotFound(w, req)
	}
	m, ok := h.meterFromPath(path)
	if !ok {
		http.NotFound(w, req)
		return
	}
	var samples []meterstat.Sample
	if h.p.SampleDirPath != "" {
		path := filepath.Join(h.p.SampleDirPath, m.SampleDir(), "manual.sample")
		sampleFile, err := meterstat.OpenSampleFile(path)
		if err != nil {
			if !os.IsNotExist(err) && err != meterstat.ErrNoSamples {
				log.Printf("cannot open manual sample file: %v", err)
			}
		} else {
			samples, err = meterstat.ReadAllSamples(sampleFile)
			sampleFile.Close()
			if err != nil {
				log.Printf("error reading samples from %q: %v", path, err)
			}
		}
	}
	p := meterTemplParams{
		Meter:   m,
		Samples: samples,
	}
	var b bytes.Buffer
	if err := meterTempl.Execute(&b, p); err != nil {
		log.Printf("meter template execution failed: %v", err)
		http.Error(w, fmt.Sprintf("template execution failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Write(b.Bytes())
}

func (h *Handler) serveSamples(w http.ResponseWriter, req *http.Request) {
	path := strings.TrimPrefix(req.URL.Path, "/samples/")
	if path == "" {
		// TODO could serve summary of meters.
		http.NotFound(w, req)
	}
	m, ok := h.meterFromPath(path)
	if !ok {
		http.NotFound(w, req)
		return
	}
	switch req.Method {
	case "GET":
		h.serveSamplesGet(w, req, m)
	case "POST":
		h.serveSamplesPost(w, req, m)
	default:
		http.Error(w, "only POST and GET allowed", http.StatusMethodNotAllowed)
	}
}

// serveSamplesGet serves GET /samples/:meter by returning all the samples available for this meter.
func (h *Handler) serveSamplesGet(w http.ResponseWriter, req *http.Request, m meterworker.Meter) {
	if h.p.SampleDirPath == "" {
		return
	}
	sdir, err := meterstat.ReadSampleDir(filepath.Join(h.p.SampleDirPath, m.SampleDir()), "*.sample")
	if err != nil {
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	f := sdir.Open()
	defer f.Close()
	meterstat.WriteSamples(w, sdir.Open())
}

// serveSamplesPost serves POST /samples/:meter by updating the manually added samples.
func (h *Handler) serveSamplesPost(w http.ResponseWriter, req *http.Request, m meterworker.Meter) {
	if h.p.SampleDirPath == "" {
		http.Error(w, "samples aren't enabled", http.StatusForbidden)
		return
	}
	req.ParseForm()
	samplesText := req.Form.Get("samples")
	samples, err := parseSamples(samplesText, h.p.TZ)
	if err != nil {
		// TODO better error page.
		http.Error(w, fmt.Sprintf("invalid samples: %v", err), http.StatusBadRequest)
		return
	}
	sampleDir := filepath.Join(h.p.SampleDirPath, m.SampleDir())
	sampleFilePath := filepath.Join(sampleDir, "manual.sample")
	if len(samples) == 0 {
		os.Remove(sampleFilePath)
		http.Redirect(w, req, "/index.html", http.StatusMovedPermanently)
		return
	}
	if err := os.MkdirAll(sampleDir, 0777); err != nil {
		http.Error(w, fmt.Sprintf("cannot make sample directory: %v", err), http.StatusInternalServerError)
		return
	}
	f, err := os.Create(sampleFilePath)
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot create sample file: %v", err), http.StatusInternalServerError)
		return
	}
	defer f.Close()
	bufw := bufio.NewWriter(f)
	defer bufw.Flush()
	n, err := meterstat.WriteSamples(bufw, meterstat.NewMemSampleReader(samples))
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot write samples to %q: %v", sampleFilePath, err), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, req, "/index.html", http.StatusMovedPermanently)
}

func parseSamples(samplesText string, tz *time.Location) ([]meterstat.Sample, error) {
	var samples []meterstat.Sample
	line := 1
	var prevSample meterstat.Sample
	for scan := bufio.NewScanner(strings.NewReader(samplesText)); scan.Scan(); line++ {
		fields := strings.Fields(scan.Text())
		if len(fields) == 0 {
			continue
		}
		// TODO could allow 2 fields (just a date) for convenience, and choose 12pm
		// as an arbitrary place.
		if len(fields) != 3 {
			return nil, fmt.Errorf("invalid number of fields on line %d", line)
		}
		t, err := time.ParseInLocation("2006-01-02 15:04", fields[0]+" "+fields[1], tz)
		if err != nil {
			return nil, fmt.Errorf("invalid time on line %d: %v", line, err)
		}
		eStr := strings.TrimSuffix(strings.ToLower(fields[2]), "kwh")
		e, err := strconv.ParseFloat(eStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid energy reading %q on line %d", fields[2], line)
		}
		if !t.After(prevSample.Time) {
			return nil, fmt.Errorf("samples must be in strict time order (line %d is before previous line)", line)
		}
		if e < prevSample.TotalEnergy {
			return nil, fmt.Errorf("energy must not go down (line %d is before previous line)", line)
		}
		sample := meterstat.Sample{
			Time:        t,
			TotalEnergy: e * 1000,
		}
		samples = append(samples, sample)
		prevSample = sample
	}
	return samples, nil
}

func (h *Handler) meterFromPath(path string) (meterworker.Meter, bool) {
	mstate := h.store.meterState()
	if path == "" || strings.Index(path, "/") != -1 || mstate == nil {
		return meterworker.Meter{}, false
	}
	for _, m := range mstate.Meters {
		if m.Addr == path {
			return m, true
		}
	}
	return meterworker.Meter{}, false
}

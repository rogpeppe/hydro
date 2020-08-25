package ndmetertest

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/rogpeppe/hydro/meterstat"
	"gopkg.in/httprequest.v1"
)

var valuesTmpl = template.Must(template.New("").Parse(`
<HTML>
<table>
	<td>Data:-</td>
	<td id='ap'>{{.SystemKW}}</td>
	<td id='rp'>10</td>
	<td id='v1'>2501</td>
	<td id='i1'>2</td>
	<td id='pf1'>0</td>
	<td id='p1'>0</td>
	<td id='v2'>2510</td>
	<td id='i2'>2</td>
	<td id='pf2'>0</td>
	<td id='p2'>0</td>
	<td id='v3'>2595</td>
	<td id='i3'>44</td>
	<td id='pf3'>1000</td>
	<td id='p3'>114</td>
	<td id='ae'>{{.SystemKWh}}</td>
	<td id='re'>25743</td>
	<td id='ascale'>2</td>
	<td id='vscale'>2</td>
	<td id='pscale'>4</td>
	<td id='escale'>5</td>
	<td id='ct'>150</td>
	<td id='nv'>480</td>
	<td id='model'>350</td>
    <td id='ps'>1</td>
	<td>-----</td>
</table>
</HTML>`[1:]))

const (
	escale = 5
	pscale = 4
)

type liveValues struct {
	SystemKW  int
	SystemKWh int
}

type Server struct {
	Addr    string
	lis     net.Listener
	mu      sync.Mutex
	power   float64
	energy  float64
	delay   time.Duration
	samples sampleSlice
}

var reqServer = &httprequest.Server{}

func NewServer(addr string) (*Server, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	srv := &Server{
		Addr: lis.Addr().String(),
		lis:  lis,
	}
	router := httprouter.New()
	for _, h := range reqServer.Handlers(srv.handler) {
		router.Handle(h.Method, h.Path, h.Handle)
	}
	go http.Serve(lis, router)
	return srv, nil
}

func (srv *Server) SetPower(power float64) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.power = power
}

func (srv *Server) SetEnergy(energy float64) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.energy = energy
}

func (srv *Server) SetDelay(delay float64) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.delay = time.Duration(delay * float64(time.Second))
}

func (srv *Server) handler(p httprequest.Params) (handler, context.Context, error) {
	return handler{srv}, p.Context, nil
}

func (srv *Server) AddSamples(samples []meterstat.Sample) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.samples = append(srv.samples, samples...)
	sort.Sort(srv.samples)
}

func (srv *Server) Close() {
	srv.lis.Close()
}

type handler struct {
	srv *Server
}

type liveValuesReq struct {
	httprequest.Route `httprequest:"GET /Values_live.shtml"`
}

func (h handler) LiveValues(p httprequest.Params, req *liveValuesReq) {
	h.srv.mu.Lock()
	delay := h.srv.delay
	h.srv.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	h.srv.mu.Lock()
	defer h.srv.mu.Unlock()
	p.Response.Header().Set("Content-Type", "text/html")
	p.Response.Header().Set("Date", time.Now().UTC().Format("Mon, 2 Jan 2006 15:04:05 MST"))
	if err := valuesTmpl.Execute(p.Response, liveValues{
		SystemKW:  int(h.srv.power/10 + 0.5),
		SystemKWh: int(h.srv.energy/100 + 0.5),
	}); err != nil {
		log.Printf("cannot execute template: %v", err)
	}
}

type energyLogReq struct {
	httprequest.Route `httprequest:"POST /Read_Energy.cgi"`
	From              timestamp `httprequest:"From,form"`
	To                timestamp `httprequest:"To,form"`
	Fmt               string    `httprequest:"Fmt,form"`
}

func (h handler) ReadEnergyLog(p httprequest.Params, req *energyLogReq) error {
	if req.Fmt != "csv" {
		return fmt.Errorf("unexpected format %q in energy log request", req.Fmt)
	}
	t0, t1 := req.From.t, req.To.t
	if t0.After(t1) {
		return fmt.Errorf("energy log read: From is before To")
	}
	h.srv.mu.Lock()
	defer h.srv.mu.Unlock()
	fmt.Fprintf(p.Response, "Date, Time, kWh, Export kWh, Counter 1, Counter 2, Counter 3\n")
	for _, s := range h.srv.samples {
		if !s.Time.Before(t0) && !s.Time.After(t1) {
			fmt.Fprintf(p.Response, "%s,%f,0,0,0,0\n", s.Time.UTC().Round(time.Second).Format("02-01-2006,15:04:05"), s.TotalEnergy/1000)
		}
	}
	return nil
}

const timeOffset = 315532800

type timestamp struct {
	t time.Time
}

func (t *timestamp) UnmarshalText(data []byte) error {
	ts, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	t.t = time.Unix(ts+timeOffset, 0).In(time.UTC)
	return nil
}

func (t timestamp) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprint(t.t.Unix() - timeOffset)), nil
}

type setPowerReq struct {
	httprequest.Route `httprequest:"PUT /v/ap"`
	Value             float64 `httprequest:"v,form"`
}

func (h handler) SetPower(req *setPowerReq) {
	h.srv.SetPower(req.Value)
}

type setDelayReq struct {
	httprequest.Route `httprequest:"PUT /delay"`
	Delay             float64 `httprequest:"delay,form"`
}

func (h handler) SetDelay(req *setDelayReq) {
	h.srv.SetDelay(req.Delay)
}

type setEnergyReq struct {
	httprequest.Route `httprequest:"PUT /v/ae"`
	Value             float64 `httprequest:"v,form"`
}

func (h handler) SetEnergy(req *setEnergyReq) {
	h.srv.SetEnergy(req.Value)
}

type sampleSlice []meterstat.Sample

func (s sampleSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s sampleSlice) Len() int {
	return len(s)
}

func (s sampleSlice) Less(i, j int) bool {
	return s[i].Time.Before(s[j].Time)
}

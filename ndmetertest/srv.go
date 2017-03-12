package ndmetertest

import (
	"context"
	"html/template"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/juju/httprequest"
	"github.com/julienschmidt/httprouter"
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
	Addr   string
	lis    net.Listener
	mu     sync.Mutex
	Power  float64
	Energy float64
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
	srv.Power = power
}

func (srv *Server) SetEnergy(energy float64) {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	srv.Energy = energy
}

func (srv *Server) handler(p httprequest.Params) (handler, context.Context, error) {
	return handler{srv}, p.Context, nil
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
	defer h.srv.mu.Unlock()
	p.Response.Header().Set("Content-Type", "text/html")
	p.Response.Header().Set("Date", time.Now().UTC().Format("Mon, 2 Jan 2006 15:04:05 MST"))
	if err := valuesTmpl.Execute(p.Response, liveValues{
		SystemKW:  int(h.srv.Power/10 + 0.5),
		SystemKWh: int(h.srv.Energy/100 + 0.5),
	}); err != nil {
		log.Printf("cannot execute template: %v", err)
	}
}

type setPowerReq struct {
	httprequest.Route `httprequest:"PUT /v/ap"`
	Value             float64 `httprequest:"v,form"`
}

func (h handler) SetPower(req *setPowerReq) {
	h.srv.SetPower(req.Value)
}

type setEnergyReq struct {
	httprequest.Route `httprequest:"PUT /v/ae"`
	Value             float64 `httprequest:"v,form"`
}

func (h handler) SetEnergy(req *setEnergyReq) {
	h.srv.SetEnergy(req.Value)
}

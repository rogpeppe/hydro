package hydroserver

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rakyll/statik/fs"
	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/history"
	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/hydroworker"
	_ "github.com/rogpeppe/hydro/statik"
)

const (
	darkDuckEgg = 0x558c89
	midDuckEgg  = 0x74afad
	orange      = 0xd9853b
	grey        = 0xececea
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Handler struct {
	store  *store
	worker *hydroworker.Worker
	mux    *http.ServeMux
}

type Params struct {
	RelayCtlAddr string
}

func New(p Params) (*Handler, error) {
	staticData, err := fs.New()
	if err != nil {
		return nil, errgo.Notef(err, "cannot get static data")
	}
	// TODO initialize the store from stored configuration.
	store, err := newStore()
	if err != nil {
		return nil, errgo.Notef(err, "cannot make store")
	}
	w, err := hydroworker.New(hydroworker.Params{
		Config:     store.CtlConfig(),
		Store:      new(history.MemStore),
		Updater:    store,
		Controller: newRelayController(p.RelayCtlAddr, ""),
		Meters:     meterReader{},
	})
	if err != nil {
		return nil, errgo.Notef(err, "cannot start worker")
	}
	h := &Handler{
		store:  store,
		mux:    http.NewServeMux(),
		worker: w,
	}
	go h.configUpdater()
	h.store.anyVal.Set(nil)
	h.mux.Handle("/static/", http.StripPrefix("/static", http.FileServer(staticData)))
	h.mux.HandleFunc("/index.html", serveIndex)
	h.mux.HandleFunc("/updates", h.serveUpdates)
	h.mux.HandleFunc("/config", h.serveConfig)
	return h, nil
}

func (h *Handler) configUpdater() {
	for {
		for w := h.store.configVal.Watch(); w.Next(); {
			h.worker.SetConfig(h.store.CtlConfig())
		}
	}
}

func (h *Handler) Close() {
	// TODO Possible race here: closing the val will cause configUpdater to
	// exit, but it might be about to make a call to the worker,
	// and method calls to the worker after it's closed will panic.
	// Decide whether to close synchronously or make method calls
	// not panic.
	h.store.anyVal.Close()
	h.store.configVal.Close()
	h.worker.Close()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Printf("request: %s %v", req.Method, req.URL)
	h.mux.ServeHTTP(w, req)
}

func (h *Handler) badRequest(w http.ResponseWriter, req *http.Request, err error) {
	log.Printf("bad request: %v", err)
	http.Error(w, fmt.Sprintf("bad request (%s %v): %v", req.Method, req.URL, err), http.StatusBadRequest)
}

func (h *Handler) ReadMeters() (hydroctl.MeterReading, error) {
	// TODO initially derive readings from current meter state.
	// TODO read actual meter readings or scrape off the web.
	return hydroctl.MeterReading{}, nil
}

type update struct {
	Meters *hydroctl.MeterReading `json:",omitempty"`
	Relays map[int]relayUpdate    `json:",omitempty"`
}

type relayUpdate struct {
	On     bool
	Since  time.Time
	Cohort string
}

func (h *Handler) serveUpdates(w http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("connection upgrade failed: %v", err)
		return
	}
	log.Printf("websocket connection made")
	for w := h.store.anyVal.Watch(); w.Next(); {
		if err := conn.WriteJSON(h.makeUpdate()); err != nil {
			log.Printf("cannot write JSON to websocket: %v", err)
			return
		}
	}
}

type clientUpdate struct {
	Relays []clientRelayInfo
}

type clientRelayInfo struct {
	Relay int
	On    bool
	Since time.Time
}

func (h *Handler) makeUpdate() clientUpdate {
	ws := h.store.WorkerState()
	var u clientUpdate
	if ws == nil {
		return u
	}
	for i, r := range ws.Relays {
		if r.Since.IsZero() && !r.On {
			continue
		}
		u.Relays = append(u.Relays, clientRelayInfo{
			Relay: i,
			On:    r.On,
			Since: r.Since,
		})
	}
	// TODO read meters
	return u
}

func serveIndex(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte(indexHTML))
}

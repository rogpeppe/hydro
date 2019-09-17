package hydroserver

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/gorilla/websocket"
	"github.com/rakyll/statik/fs"
	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/googlecharts"
	"github.com/rogpeppe/hydro/history"
	"github.com/rogpeppe/hydro/hydroctl"
	"github.com/rogpeppe/hydro/hydroworker"
	_ "github.com/rogpeppe/hydro/statik"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Handler struct {
	store      *store
	worker     *hydroworker.Worker
	controller *relayCtl
	mux        *http.ServeMux
	history    *history.DiskStore
}

type Params struct {
	RelayAddrPath   string
	ConfigPath      string
	MeterConfigPath string
	HistoryPath     string
}

func New(p Params) (*Handler, error) {
	staticData, err := fs.New()
	if err != nil {
		return nil, errgo.Notef(err, "cannot get static data")
	}
	store, err := newStore(p.ConfigPath, p.MeterConfigPath)
	if err != nil {
		return nil, errgo.Notef(err, "cannot make store")
	}
	historyStore, err := history.NewDiskStore(p.HistoryPath, time.Now().Add(-7*24*time.Hour))
	if err != nil {
		return nil, errgo.Notef(err, "cannot open history file")
	}
	relayCtlConfigStore := &relayCtlConfigStore{
		path: p.RelayAddrPath,
	}
	controller := newRelayController(relayCtlConfigStore)

	hwp := hydroworker.Params{
		Config:     store.CtlConfig(),
		Store:      historyStore,
		Updater:    store,
		Controller: controller,
		Meters:     store,
	}
	w, err := hydroworker.New(hwp)
	if err != nil {
		return nil, errgo.Notef(err, "cannot start worker")
	}
	h := &Handler{
		store:      store,
		mux:        http.NewServeMux(),
		worker:     w,
		controller: controller,
		history:    historyStore,
	}
	go h.configUpdater()
	h.store.anyVal.Set(nil)
	h.mux.Handle("/", gziphandler.GzipHandler(http.FileServer(staticData)))
	h.mux.HandleFunc("/updates", h.serveUpdates)
	h.mux.HandleFunc("/history.json", h.serveHistory)
	h.mux.HandleFunc("/config", h.serveConfig)
	h.mux.Handle("/api/", newAPIHandler(h))
	// Let's see what's going on.
	h.mux.HandleFunc("/debug/pprof/", pprof.Index)
	h.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	h.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	h.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	h.mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
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

func badRequest(w http.ResponseWriter, req *http.Request, err error) {
	log.Printf("bad request: %v", err)
	http.Error(w, fmt.Sprintf("bad request (%s %v): %v", req.Method, req.URL, err), http.StatusBadRequest)
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

type historyRecord struct {
	Name  string
	Start time.Time
	End   time.Time
}

func (h *Handler) serveHistory(w http.ResponseWriter, req *http.Request) {
	ws := h.store.WorkerState()
	if ws == nil {
		http.Error(w, "no current relay information available", http.StatusInternalServerError)
		return
	}
	cfg := h.store.CtlConfig()
	now := time.Now()
	offTimes := make([]time.Time, hydroctl.MaxRelayCount)
	for i := range offTimes {
		if ws.State.IsSet(i) {
			offTimes[i] = now
		}
	}
	limit := now.Add(-7 * 24 * time.Hour)
	var records []historyRecord
	iter := h.history.ReverseIter()
	for iter.Next() {
		e := iter.Item()
		if e.Time.Before(limit) {
			break
		}
		if e.On {
			if offt := offTimes[e.Relay]; !offt.IsZero() {
				records = append(records, historyRecord{
					// TODO use relay number only when needed for disambiguation.
					Name:  fmt.Sprintf("%d: %s", e.Relay, cfg.Relays[e.Relay].Cohort),
					Start: e.Time,
					End:   offt,
				})
				offTimes[e.Relay] = time.Time{}
			}
		} else {
			offTimes[e.Relay] = e.Time
		}
	}
	// Give starting times to all the periods that start before the limit.
	for i, offt := range offTimes {
		if !offt.IsZero() {
			records = append(records, historyRecord{
				// TODO use relay number only when needed for disambiguation.
				Name:  fmt.Sprintf("%d: %s", i, cfg.Relays[i].Cohort),
				Start: limit,
				End:   offt,
			})
		}
	}
	data, err := json.Marshal(googlecharts.NewDataTable(records))
	if err != nil {
		http.Error(w, fmt.Sprintf("cannot marshal data table: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

type clientUpdate struct {
	Relays []clientRelayInfo
	Meters *clientMeterInfo
}

type clientRelayInfo struct {
	Cohort string
	Relay  int
	On     bool
	Since  string
}

type clientSample struct {
	TimeLag     string
	Power       float64
	TotalEnergy float64
}

type clientMeterInfo struct {
	Chargeable hydroctl.PowerChargeable
	Use        hydroctl.PowerUse
	Meters     []meter
	Samples    map[string]clientSample
}

// expectedMaxRoundTrip holds the maximum duration we might normally expect
// a meter request to take. If we've got a sample that's older than the allowed lag
// plus the round trip time, we consider that it's useful to display the lag to the user
// as a hint that all is not well.
const expectedMaxRoundTrip = time.Second

func (h *Handler) makeUpdate() clientUpdate {
	ws := h.store.WorkerState()
	cfg := h.store.CtlConfig()
	meters := h.store.meterState()
	var u clientUpdate
	samples := make(map[string]clientSample)
	for addr, s := range meters.Samples {
		// Allow 50% extra time for a round trip when the allowed lag is long,
		// or a fairly arbitrary constant when it's short. We should probably
		// do a bit better than this and estimate the usual round trip time so
		// that we send a request sufficiently in advance of the allowed-lag
		// deadline that it's rare to overrun it.
		allowedLag := s.AllowedLag * 3 / 2
		if allowedLag < expectedMaxRoundTrip {
			allowedLag = expectedMaxRoundTrip
		}
		samples[addr] = clientSample{
			TimeLag:     lag(s.Time, allowedLag, meters.Time),
			Power:       s.ActivePower,
			TotalEnergy: s.TotalEnergy,
		}
	}
	u.Meters = &clientMeterInfo{
		Chargeable: meters.Chargeable,
		Use:        meters.Use,
		Meters:     meters.Meters,
		Samples:    samples,
	}
	if ws == nil || len(ws.Relays) == 0 {
		u.Relays = []clientRelayInfo{} // be nice to JS and don't give it null.
		return u
	}
	for i, r := range ws.Relays {
		if r.Since.IsZero() && !r.On {
			continue
		}
		cohort := ""
		if cfg != nil && len(cfg.Relays) > i {
			cohort = cfg.Relays[i].Cohort
		}
		var since string
		now := time.Now()
		switch howlong := now.Sub(r.Since); {
		case howlong > 6*24*time.Hour:
			since = r.Since.Format("2006-01-02 15:04")
		case r.Since.Day() != now.Day():
			since = r.Since.Format("Mon 15:04")
		default:
			since = r.Since.Format("15:04:05")
		}

		u.Relays = append(u.Relays, clientRelayInfo{
			Cohort: cohort,
			Relay:  i,
			On:     r.On,
			Since:  since,
		})
	}
	return u
}

// lag returns a human-readable representation of the lag for
// a meter reading that was acquired at time t0 with the given
// allowed lag, when the result was returned at time t1.
func lag(t0 time.Time, allowedLag time.Duration, t1 time.Time) string {
	d := t1.Sub(t0)
	if d <= allowedLag {
		return ""
	}
	var q time.Duration
	switch {
	case d < time.Minute:
		q = time.Millisecond
	default:
		q = time.Second
	}
	return d.Round(q).String()
}

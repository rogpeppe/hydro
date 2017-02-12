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
}

type MeterLocation int

const (
	_ MeterLocation = iota
	LocGenerator
	LocHere
	LocNeighbour
)

// Meter holds a meter that can be read to find out what
// the system is doing.
type Meter struct {
	Name     string
	Location MeterLocation
	Addr     string // host:port
}

type Params struct {
	RelayAddrPath string
	ConfigPath    string
	HistoryPath   string
	Meters        []Meter
}

func New(p Params) (*Handler, error) {
	staticData, err := fs.New()
	if err != nil {
		return nil, errgo.Notef(err, "cannot get static data")
	}
	store, err := newStore(p.ConfigPath)
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
	}
	go h.configUpdater()
	h.store.anyVal.Set(nil)
	h.mux.Handle("/static/", http.StripPrefix("/static", http.FileServer(staticData)))
	h.mux.HandleFunc("/", h.serveSlash)
	h.mux.HandleFunc("/index.html", serveIndex)
	h.mux.HandleFunc("/updates", h.serveUpdates)
	h.mux.HandleFunc("/config", h.serveConfig)
	h.mux.Handle("/api/", newAPIHandler(h))
	return h, nil
}

func (h *Handler) serveSlash(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		http.NotFound(w, req)
		return
	}
	http.Redirect(w, req, "/index.html", http.StatusMovedPermanently)
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

type update struct {
	Meters *MeterState         `json:",omitempty"`
	Relays map[int]relayUpdate `json:",omitempty"`
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
	Cohort string
	Relay  int
	On     bool
	Since  time.Time
}

func (h *Handler) makeUpdate() clientUpdate {
	ws := h.store.WorkerState()
	cfg := h.store.CtlConfig()
	var u clientUpdate
	if ws == nil || len(ws.Relays) == 0 {
		return clientUpdate{
			Relays: []clientRelayInfo{}, // be nice to JS and don't give it null.
		}
	}
	for i, r := range ws.Relays {
		if r.Since.IsZero() && !r.On {
			continue
		}
		cohort := ""
		if cfg != nil && len(cfg.Relays) > i {
			cohort = cfg.Relays[i].Cohort
		}
		u.Relays = append(u.Relays, clientRelayInfo{
			Cohort: cohort,
			Relay:  i,
			On:     r.On,
			Since:  r.Since,
		})
	}
	// TODO read meters
	return u
}

func serveIndex(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte(indexHTML))
}

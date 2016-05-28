package hydroserver

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	stdpath "path"
	"sort"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/juju/httprequest"
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

var initialState = &State{
	Cohorts: map[string]*Cohort{
		"cohort0": {
			Id:       "cohort0",
			Index:    0,
			Relays:   "0",
			Title:    "Spare room",
			MaxPower: "0kW",
			Mode:     "in-use",
			InUseSlots: []Slot{{
				Start:        "1h",
				SlotDuration: "5h",
				Kind:         "at least",
				Duration:     "5h",
			}, {
				Start:        "7h",
				SlotDuration: "1h",
				Kind:         "exactly",
				Duration:     "20m",
			}},
		},
		"cohort1": {
			Id:       "cohort1",
			Index:    1,
			Relays:   "1",
			Title:    "Number 8",
			MaxPower: "3kW",
			Mode:     "not-in-use",
			NotInUseSlots: []Slot{{
				Start:        "1h",
				SlotDuration: "5h",
				Kind:         "at least",
				Duration:     "5h",
			}},
		},
		"cohort2": {
			Id:       "cohort2",
			Index:    2,
			Relays:   "2 3",
			Title:    "Test",
			MaxPower: "5kW",
			Mode:     "in-use",
			InUseSlots: []Slot{{
				Start:        "16h",
				SlotDuration: "1h",
				Kind:         "exactly",
				Duration:     "1h",
			}},
		},
	},
}

type Handler struct {
	store  *store
	worker *hydroworker.Worker
	mux    *http.ServeMux
}

type NewParams struct {
	RelayCtlAddr string
}

func New(p NewParams) (*Handler, error) {
	staticData, err := fs.New()
	if err != nil {
		return nil, errgo.Notef(err, "cannot get static data")
	}
	store, err := newStore(initialState)
	if err != nil {
		return nil, errgo.Notef(err, "cannot make store")
	}
	w, err := hydroworker.New(hydroworker.NewParams{
		Config:     store.relayConfig,
		Store:      new(history.MemStore),
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
	h.store.val.Set(nil)
	h.mux.Handle("/static/", http.StripPrefix("/static", http.FileServer(staticData)))
	h.mux.HandleFunc("/index.html", serveIndex)
	h.mux.HandleFunc("/updates", h.serveUpdates)
	h.mux.HandleFunc("/commit", h.serveCommit)
	h.mux.HandleFunc("/store/", h.serveStore)
	h.mux.HandleFunc("/basic", h.serveBasic)
	return h, nil
}

func (h *Handler) configUpdater() {
	for {
		for w := h.store.val.Watch(); w.Next(); {
			h.store.mu.Lock()
			cfg := h.store.relayConfig
			h.store.mu.Unlock()
			h.worker.SetConfig(cfg)
		}
	}
}

func (h *Handler) Close() {
	// TODO Possible race here: closing the val will cause configUpdater to
	// exit, but it might be about to make a call to the worker,
	// and method calls to the worker after it's closed will panic.
	// Decide whether to close synchronously or make method calls
	// not panic.
	h.store.val.Close()
	h.worker.Close()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Printf("request: %s %v", req.Method, req.URL)
	h.mux.ServeHTTP(w, req)
}

func (h *Handler) serveStore(w http.ResponseWriter, req *http.Request) {
	log.Printf("store %s %s", req.Method, req.URL.Path)
	path := strings.TrimPrefix(stdpath.Clean(req.URL.Path), "/store")
	switch req.Method {
	case "PUT":
		data, _ := ioutil.ReadAll(req.Body)
		log.Printf("put %s", data)
		if err := h.store.Put(path, data); err != nil {
			h.badRequest(w, req, errgo.Notef(err, "cannot put"))
			return
		}
		h.store.mu.Lock()
		hydroctl.Debug = h.store.state.Debug
		h.store.mu.Unlock()
	case "GET":
		v, err := h.store.Get(path)
		if err != nil {
			h.badRequest(w, req, errgo.Notef(err, "cannot get"))
			return
		}
		httprequest.WriteJSON(w, http.StatusOK, v)
	case "DELETE":
		if err := h.store.Delete(path); err != nil {
			h.badRequest(w, req, errgo.Notef(err, "cannot delete"))
			return
		}
	default:
		h.badRequest(w, req, errgo.New("bad method"))
	}
}

func (h *Handler) serveCommit(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		h.badRequest(w, req, errgo.New("bad method"))
		return
	}
	log.Printf("commit state")
	if err := h.store.Commit(); err != nil {
		http.Error(w, fmt.Sprintf("cannot commit: %v", err), http.StatusInternalServerError)
	}
	http.Redirect(w, req, "/index.html", http.StatusMovedPermanently)
}

func (h *Handler) badRequest(w http.ResponseWriter, req *http.Request, err error) {
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
	for w := h.store.val.Watch(); w.Next(); {
		h.store.mu.Lock()

		cohorts := cohortSlice(h.store.state.Cohorts)
		err := conn.WriteJSON(struct {
			Cohorts []*Cohort
			Debug   bool
		}{
			Cohorts: cohorts,
			Debug:   h.store.state.Debug,
		})
		h.store.mu.Unlock()
		if err != nil {
			log.Printf("cannot write JSON to websocket: %v", err)
			return
		}
	}
}

// TODO remove this function - having a different representation confuses things.
func cohortSlice(cohorts map[string]*Cohort) []*Cohort {
	slice := make([]*Cohort, 0, len(cohorts))
	for _, c := range cohorts {
		slice = append(slice, c)
	}
	sort.Sort(cohortsByIndex(slice))
	return slice
}

type cohortsByIndex []*Cohort

func (c cohortsByIndex) Len() int {
	return len(c)
}
func (c cohortsByIndex) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}
func (c cohortsByIndex) Less(i, j int) bool {
	c0, c1 := c[i], c[j]
	if c0.Index != c1.Index {
		return c0.Index < c1.Index
	}
	// Shouldn't happen as we should keep consistent index.
	return c0.Title < c1.Title
}

func serveIndex(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte(htmlPage))
}

// from http://stackoverflow.com/questions/25886660/bootstrap-with-react-accordion-wont-work
//var WontWorkPanel = React.createClass({
//  render: function() {
//    return this.transferPropsTo(
//      <ReactBootstrap.Panel header={"WontWork " + this.props.key} key={this.props.key}>
//        Anim pariatur cliche reprehenderit, enim eiusmod high life
//        accusamus terry richardson ad squid. 3 wolf moon officia aute,
//        non cupidatat skateboard dolor brunch. Food truck quinoa nesciunt
//        laborum eiusmod. Brunch 3 wolf moon tempor, sunt aliqua pu
//      </ReactBootstrap.Panel>
//    );
//  }
//});

//
//
//const accordionInstance = (
//  <Accordion>
//    <Panel header="Collapsible Group Item #1" eventKey="1">
//      Anim pariatur cliche reprehenderit, enim eiusmod high life accusamus terry richardson ad squid. 3 wolf moon officia aute, non cupidatat skateboard dolor brunch. Food truck quinoa nesciunt laborum eiusmod. Brunch 3 wolf moon tempor, sunt aliqua put a bird on it squid single-origin coffee nulla assumenda shoreditch et. Nihil anim keffiyeh helvetica, craft beer labore wes anderson cred nesciunt sapiente ea proident. Ad vegan excepteur butcher vice lomo. Leggings occaecat craft beer farm-to-table, raw denim aesthetic synth nesciunt you probably haven't heard of them accusamus labore sustainable VHS.
//    </Panel>
//    <Panel header="Collapsible Group Item #2" eventKey="2">
//      Anim pariatur cliche reprehenderit, enim eiusmod high life accusamus terry richardson ad squid. 3 wolf moon officia aute, non cupidatat skateboard dolor brunch. Food truck quinoa nesciunt laborum eiusmod. Brunch 3 wolf moon tempor, sunt aliqua put a bird on it squid single-origin coffee nulla assumenda shoreditch et. Nihil anim keffiyeh helvetica, craft beer labore wes anderson cred nesciunt sapiente ea proident. Ad vegan excepteur butcher vice lomo. Leggings occaecat craft beer farm-to-table, raw denim aesthetic synth nesciunt you probably haven't heard of them accusamus labore sustainable VHS.
//    </Panel>
//    <Panel header="Collapsible Group Item #3" eventKey="3">
//      Anim pariatur cliche reprehenderit, enim eiusmod high life accusamus terry richardson ad squid. 3 wolf moon officia aute, non cupidatat skateboard dolor brunch. Food truck quinoa nesciunt laborum eiusmod. Brunch 3 wolf moon tempor, sunt aliqua put a bird on it squid single-origin coffee nulla assumenda shoreditch et. Nihil anim keffiyeh helvetica, craft beer labore wes anderson cred nesciunt sapiente ea proident. Ad vegan excepteur butcher vice lomo. Leggings occaecat craft beer farm-to-table, raw denim aesthetic synth nesciunt you probably haven't heard of them accusamus labore sustainable VHS.
//    </Panel>
//  </Accordion>
//);
//

// -- we'd quite like a checkbox (original motivation: debug, but
// good for other things too)
//
//React.render(accordionInstance, mountNode);
//	var CheckBox = React.createClass({
//		render: function() {
//			return <input
//				type="checkbox"
//				checked={this.props.checked}
//				onChange={this.onChange.bind(this)}
//			/>;
//		},
//		onChange: function(event) {
//
//			XXX what to do here?
//			where do we get the checked state from?
//			can we avoid using react state?
//			console.log("debug ", this.state.checked)
//			$.ajax("/debug?on=" + (this.state.checked ? "1" : "0"), {
//				method: "PUT",
//				success: function() {
//					console.log("done PUT debug");
//				},
//				error: function(xhr) {
//					alert("PUT /debug failed; response text: " + xhr.responseText);
//				},
//			});
//		})
//	}

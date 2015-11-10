package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/juju/httprequest"
	"github.com/juju/utils/voyeur"
	"github.com/rakyll/statik/fs"

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

type State struct {
	maxCohortId int
	Cohorts     map[string]*Cohort
}

type Cohort struct {
	Id            string // Unique; always increases.
	Index         int    // Display index.
	Title         string
	Relays        []int
	MaxPower      string
	Status        string
	ActiveSlots   []Slot
	InactiveSlots []Slot
}

type Slot struct {
	StartTime string
	EndTime   string
	Condition string
	Duration  string
}

var initialState = State{
	maxCohortId: 1,
	Cohorts: map[string]*Cohort{
		"cohort0": {
			Id:       "cohort0",
			Index:    0,
			Relays:   []int{0},
			Title:    "Spare room",
			MaxPower: "0kW",
			Status:   "active",
			ActiveSlots: []Slot{{
				StartTime: "0100",
				EndTime:   "0600",
				Condition: ">=",
				Duration:  "5 hours",
			}, {
				StartTime: "0700",
				EndTime:   "0800",
				Condition: "==",
				Duration:  "20 mins",
			}},
		},
		"cohort1": {
			Id:       "cohort1",
			Index:    1,
			Relays:   []int{1},
			Title:    "Number 8",
			MaxPower: "3kW",
			Status:   "inactive",
			InactiveSlots: []Slot{{
				StartTime: "0100",
				EndTime:   "0600",
				Condition: ">=",
				Duration:  "5 hours",
			}},
		},
	},
}

func main() {
	staticData, err := fs.New()
	if err != nil {
		log.Fatal(err)
	}
	h := &handler{
		store: &store{
			state: initialState,
		},
	}
	h.store.val.Set(nil)
	mux := http.DefaultServeMux
	mux.Handle("/static/", http.StripPrefix("/static", http.FileServer(staticData)))
	mux.HandleFunc("/index.html", serveIndex)
	mux.HandleFunc("/updates", h.serveUpdates)
	mux.HandleFunc("/change", h.serveChange)
	mux.HandleFunc("/store/", h.serveStore)

	log.Printf("listening on :8081")
	err = http.ListenAndServe(":8081", nil)
	log.Fatal(err)
}

type handler struct {
	store *store
}

func (h *handler) serveChange(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	log.Printf("POST %s", req.URL)
	req.ParseForm()
	index := req.Form.Get("attr")
	val := req.Form.Get("value")
	h.store.mu.Lock()
	defer h.store.mu.Unlock()
	h.store.state.Cohorts[index].Title = val
	h.store.val.Set(nil)
}

func (h *handler) serveStore(w http.ResponseWriter, req *http.Request) {
	log.Printf("store %s %s", req.Method, req.URL.Path)
	path := strings.TrimPrefix(req.URL.Path, "/store/")
	h.store.mu.Lock()
	defer h.store.mu.Unlock()
	switch req.Method {
	case "PUT":
		data, _ := ioutil.ReadAll(req.Body)
		log.Printf("put %s", data)
		if err := putVal(h.store.state, path, data); err != nil {
			http.Error(w, fmt.Sprintf("cannot put: %v", err), http.StatusBadRequest)
			return
		}
		h.store.val.Set(nil)
	case "GET":
		v, err := getVal(h.store.state, path)
		if err != nil {
			http.Error(w, fmt.Sprintf("cannot get: %v", err), http.StatusBadRequest)
			return
		}
		httprequest.WriteJSON(w, http.StatusOK, v)
	case "DELETE":
		err := deleteVal(h.store.state, path)
		if err != nil {
			http.Error(w, fmt.Sprintf("cannot delete: %v", err), http.StatusBadRequest)
			return
		}
		h.store.val.Set(nil)
	default:
		http.Error(w, fmt.Sprintf("bad method"), http.StatusBadRequest)
	}
}

type store struct {
	val voyeur.Value

	mu    sync.Mutex
	state State
}

func (h *handler) serveUpdates(w http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("connection upgrade failed: %v", err)
		return
	}
	log.Printf("websocket connection made")
	for w := h.store.val.Watch(); w.Next(); {
		h.store.mu.Lock()

		err := conn.WriteJSON(cohortSlice(h.store.state.Cohorts))
		h.store.mu.Unlock()
		if err != nil {
			log.Printf("cannot write JSON to websocket: %v", err)
			return
		}
	}
}

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

var htmlPage = `<!DOCTYPE html>
<html>
	<head>
		<title>Page Title</title>
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<!-- Bootstrap -->
		<link rel="stylesheet" href="/static/bootstrap-3.3.5-dist/css/bootstrap.css">
		<link rel="stylesheet" href="/static/bootstrap-3.3.5-dist/css/bootstrap-theme.css">
		<script src="/static/jquery.js"></script>
		<script src="/static/bootstrap-3.3.5-dist/js/bootstrap.min.js"></script>
		<script src="/static/react/react.js"></script>
		<script src="/static/react/react-dom.js"></script>
		<script src="/static/react-bootstrap-0.27.3.js"></script>
		<script src="/static/babel-browser.min.js"></script>
		<script src="/static/reconnecting-websocket.js"></script>
		<script src="/static/es6-promise.min.js"></script>
		<style type="text/css">
			html, body {
				padding-bottom: 70px;
				width: 900px;
				max-width: 900px;
				margin: 0 auto;
			}
			
			.content {margin:10px;}
			.cohortRelays {
				font-size: 150%;
				padding-right: 20px;
			}
			.cohortTitle {
				font-size: 150%;
				padding-left: 20px;
				padding-right: 20px;
				display: block;
				background-color: #74afad;
			}
			.cohortStatus {
				font-size: 150%;
				padding-left: 20px;
				padding-right: 20px;
			}
			.slotTitle {
				font-size: 120%;
				padding-left: 30px;
			}
			.cohortMaxPower {
				font-size: 150%;
				padding-left: 20px;
				padding-right: 20px;
			}
			.slot {
				font-size: 120%;
			}
		</style>
	</head>

	<body >
		<script type="text/babel">
		` + prog + `
		</script>
		<div id="topLevel"></div>
	</body>
</html>
`

var prog = `
	var Accordion = ReactBootstrap.Accordion
	var Panel = ReactBootstrap.Panel
	var Slot = React.createClass({
		render: function() {
			var data = this.props.data;
			return <div className="slot">{data.StartTime} to {data.EndTime} {data.Condition} for {data.Duration}</div>
		}
	});
	var EditOnClick = React.createClass({
		getInitialState: function() {
			console.log("getting initial state")
			return {value: ""};
		},
		render: function() {
			if(this.state.editing){
				return <input
					type="text"
					ref={elem => {this.inputElem = elem }}
					value={this.state.value}
					className={this.props.className}
					onChange={this.handleChange}
					onBlur={this.handleOnBlur}
				/>;
			} else {
				return <div
					className={this.props.className}
					onClick={this.handleClick}>
					{this.props.value}
				</div>;
			}
		},
		handleOnBlur: function(event) {
			this.setState({editing: false})
		},
		handleChange: function(event) {
			this.setState({value: event.target.value});
			$.ajax("/store/Cohorts/" + this.props.attr + "/Title", {
				method: "PUT",
				data: JSON.stringify(event.target.value),
				// TODO JSON content type
				success: function() {
					console.log("done PUT")
				},
				error: function() {
					console.log("PUT failed")
				},
			})
		},
		handleClick: function(event) {
			this.setState({editing: true});
		},
		componentDidUpdate: function() {
			if (this.inputElem != null) {
				this.inputElem.focus()
			}
		},
	});
	var Slots = React.createClass({
		render: function() {
			if(!this.props.slots){
				console.log("Slots.render -> nothing")
				return <span/>
			}
			console.log("Slots.render -> something")
			return <div>
				<div className="slotTitle">{this.props.title}</div>
				<ul>{
					this.props.slots.map(function(slot){
						return <li key={slot.StartTime}><Slot data={slot}/></li>
					})
				}</ul>
			</div>
		}
	});
	var Cohort = React.createClass({
		render: function() {
			var data = this.props.data
			return <div key={data.Id}>
				<EditOnClick className="cohortTitle" attr={data.Id} value={data.Title} />
				<span className="cohortStatus">status: {data.Status}</span>
				<span className="cohortMaxPower">max power: {data.MaxPower}</span>
				<Slots title="Active slots" slots={data.ActiveSlots}/>
				<Slots title="Inactive slots" slots={data.InactiveSlots}/>
			</div>
		}
	});
	var HydroControl = React.createClass({
		render: function() {
			return <div className="cohortControl">{
				this.props.cohorts.map(function(cohort){
					return <Cohort data={cohort} key={cohort.Id}/>
				})
			}</div>
		}
	});
	function wsURL(path) {
		var loc = window.location, scheme;
		if (loc.protocol === "https:") {
			scheme = "wss:";
		} else {
			scheme = "ws:";
		}
		return scheme + "//" + loc.host + path;
	}

	var socket = new ReconnectingWebSocket(wsURL("/updates"));
	socket.onmessage = function(event) {
		var m = JSON.parse(event.data);
		console.log("message", event.data)
		ReactDOM.render(<HydroControl cohorts={m}/>, document.getElementById("topLevel"));
	};
`

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
//React.render(accordionInstance, mountNode);

package main

import (
	"fmt"
	"time"
	"log"
	"net/http"

	_ "github.com/rogpeppe/hydro/statik"

	"github.com/gorilla/websocket"
	"github.com/rakyll/statik/fs"
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

type Relay struct {
	Number      int
	Title       string
	MaxPower    string
	Status      string
	ActiveSlots []Slot
	InactiveSlots []Slot
}

type Slot struct {
	StartTime string
	EndTime   string
	Condition string
	Duration  string
}

func main() {
	staticData, err := fs.New()
	if err != nil {
		log.Fatal(err)
	}
	mux := http.DefaultServeMux
	mux.Handle("/static/", http.StripPrefix("/static", http.FileServer(staticData)))
	mux.HandleFunc("/index.html", serveIndex)
	mux.HandleFunc("/updates", serveUpdates)
	mux.HandleFunc("/change", serveChange)

	log.Printf("listening on localhost:8081")
	err = http.ListenAndServe("localhost:8081", nil)
	log.Fatal(err)
}

func serveChange(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	log.Printf("POST %s", req.URL)
}

func serveUpdates(w http.ResponseWriter, req *http.Request) {
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		log.Printf("connection upgrade failed: %v", err)
		return
	}
	data := []Relay{{
		Number:   0,
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
			EndTime: "0800",
			Condition: "==",
			Duration: "20 mins",
		}},
	}, {
		Number:   1,
		Title:    "Number 8",
		MaxPower: "3kW",
		Status:   "inactive",
		InactiveSlots: []Slot{{
			StartTime: "0100",
			EndTime:   "0600",
			Condition: ">=",
			Duration:  "5 hours",
		}},
	}}
	log.Printf("websocket connection made")
	for i := 0; i < 2 ; i++ {
		data[0].MaxPower = fmt.Sprintf("%dkW", i)
		if err := conn.WriteJSON(data); err != nil {
			log.Printf("cannot write JSON to websocket: %v", err)
			return
		}
		log.Printf("wrote message")
		time.Sleep(time.Second)
	}
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
			.relayNumber {
				font-size: 150%;
				padding-right: 20px;
			}
			.relayTitle {
				font-size: 150%;
				padding-left: 20px;
				padding-right: 20px;
				display: block;
			}
			.relayStatus {
				font-size: 150%;
				padding-left: 20px;
				padding-right: 20px;
			}
			.relayMaxPower {
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
			$.ajax("/change?attr=" + event.target.value, {
				method: "POST",
				success: function() {
					console.log("done POST")
				},
				error: function() {
					console.log("POST failed")
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
	})
	var Relay = React.createClass({
		render: function() {
			var data = this.props.data;
			// why doesn't this work?
			// var {data, ...otherProps} = this.props;
			var otherProps = jQuery.extend({}, this.props)
			delete otherProps.data
			console.log("Relay created with other props", otherProps);
			var slots = [];
			if(data.Status === "active"){
				slots = data.ActiveSlots;
			} else if(data.Status === "inactive"){
				slots = data.InactiveSlots;
			}
			return <Panel bsStyle="primary" header={"Relay " + data.Number} eventKey={data.Number} {... otherProps}>
				<div className="row">
					<div className="relayHeader col-sm-8 col-offset-2">
						<span className="relayNumber">{data.Number}.</span>
						<EditOnClick className="relayTitle" value={data.Title} />
						<span className="relayStatus">status: {data.Status}</span>
						<span className="relayMaxPower">max power: {data.MaxPower}</span>
					</div>
				</div>
				<ul>{
					slots.map(function(slot){
						return <li><Slot data={slot} key={slot.StartTime}/></li>
					})
				}</ul>
			</Panel>
		}
	})
	var HydroControl = React.createClass({
		render: function() {
			return <Accordion>{
				this.props.relays.map(function(relay){
					return <Relay key={relay.Number} data={relay}/>
				})
			}</Accordion>
		}
	})
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
		ReactDOM.render(<HydroControl relays={m}/>, document.getElementById("topLevel"));
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

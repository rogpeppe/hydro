function kWfmt(watts) {
	return (watts / 1000).toFixed(3) + "kW"
};
function wsURL(path) {
	var loc = window.location, scheme;
	if (loc.protocol === "https:") {
		scheme = "wss:";
	} else {
		scheme = "ws:";
	}
	return scheme + "//" + loc.host + path;
};
var Relays = React.createClass({
	render: function() {
		return <table>
			<thead>
				<tr><th>Cohort</th><th>Relay</th><th>Status</th><th>Since</th></tr>
			</thead>
			<tbody>
			{
				this.props.relays && this.props.relays.map(function(relay){
					return <tr><td>{relay.Cohort}</td><td><a href={"/relay/" + relay.Relay}>{relay.Relay}</a></td><td>{relay.On ? "on" : "off"}</td><td>{relay.Since}</td></tr>
				})
			}
			</tbody>
		</table>
	}
})
var Meters = React.createClass({
	render: function() {
		var meters = this.props.meters
		return <div>
			<table>
			<thead>
				<tr><th>Name</th><th>Chargeable power</th></tr>
			</thead>
			<tbody>
				<tr><td>power exported to grid</td><td>{kWfmt(meters.Chargeable.ExportGrid)}</td></tr>
				<tr><td>export power used by Aliday</td><td>{kWfmt(meters.Chargeable.ExportNeighbour)}</td></tr>
				<tr><td>export power used by Drynoch</td><td>{kWfmt(meters.Chargeable.ExportHere)}</td></tr>
				<tr><td>import power used by Aliday</td><td>{kWfmt(meters.Chargeable.ImportNeighbour)}</td></tr>
				<tr><td>import power used by Drynoch</td><td>{kWfmt(meters.Chargeable.ImportHere)}</td></tr>
			</tbody>
			</table>
			<table>
			<thead>
				<tr><th>Meter name</th><th>Address</th><th>Current power (kW)</th><th>Time lag</th></tr>
			</thead>
			<tbody>
			{
				meters.Meters && meters.Meters.map(function(meter){
					var sample;
					if(meters.Samples){
						sample = meters.Samples[meter.Addr];
					}
					var sample = meters.Samples && meters.Samples[meter.Addr];
					return <tr>
						<td>{meter.Name}</td>
						<td><a href={"http://" + meter.Addr}>{meter.Addr}</a></td>
						<td>{sample ? kWfmt(sample.Power) : "n/a"}</td>
						<td>{sample ? sample.TimeLag : ""}</td>
					</tr>
				})
			}
			</tbody>
			</table>
		</div>
	}
})
var socket = new ReconnectingWebSocket(wsURL("/updates"));
socket.onmessage = function(event) {
	var m = JSON.parse(event.data);
	console.log("message", event.data);
	var toplev = document.getElementById("topLevel")
	console.log("toplev", toplev, "document", document)
	ReactDOM.render(
		<div>
			<a href="/config">Change configuration</a>
			<p/>
			<Relays relays={m.Relays}/>
			<Meters meters={m.Meters}/>
		</div>, toplev);
};


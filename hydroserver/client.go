package hydroserver

var indexHTML = `<!DOCTYPE html>
	<head>
		<title>Page Title</title>
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<!-- Bootstrap -->
		<link rel="stylesheet" href="/static/bootstrap-3.3.5-dist/css/bootstrap.min.css">
		<link rel="stylesheet" href="/static/bootstrap-3.3.5-dist/css/bootstrap-theme.min.css">
		<script src="/static/jquery-1.11.1.min.js"></script>
		<script src="/static/bootstrap-3.3.5-dist/js/bootstrap.min.js"></script>
		<script src="/static/react/react.min.js"></script>
		<script src="/static/react/react-dom.min.js"></script>
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
			td {
				padding: 10px;
			}
			th {
				padding: 10px;
			}
			
			.content {margin:10px;}
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
	function kWfmt(watts) {
		return (watts / 1000).toFixed(3) + "kW"
	}
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
					<tr><th>Meter name</th><th>Address</th><th>Current power (kW)</th></tr>
				</thead>
				<tbody>
				{
					meters.Meters && meters.Meters.map(function(meter){
						return <tr>
							<td>{meter.Name}</td>
							<td><a href={"http://" + meter.Addr}>{meter.Addr}</a></td>
							<td>{
								meters.Samples[meter.Addr] === undefined ? "n/a" : kWfmt(meters.Samples[meter.Addr].ActivePower)
							}</td>
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
		console.log("message", event.data)
		ReactDOM.render(
			<div>
				<a href="/config">Change configuration</a>
				<p/>
				<Relays relays={m.Relays}/>
				<Meters meters={m.Meters}/>
			</div>, document.getElementById("topLevel"));
	};
`

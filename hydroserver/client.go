package hydroserver

var indexHTML = `<!DOCTYPE html>
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
	var socket = new ReconnectingWebSocket(wsURL("/updates"));
	socket.onmessage = function(event) {
		var m = JSON.parse(event.data);
		console.log("message", event.data)
		ReactDOM.render(
			<div>
				<a href="/config">Change configuration</a>
				<p/>
				<Relays relays={m.Relays}/>
			</div>, document.getElementById("topLevel"));
	};
`

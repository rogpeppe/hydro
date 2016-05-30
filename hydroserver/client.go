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
			
			.content {margin:10px;}
		</style>
	</head>

	<body >
		<a href="/config">Change configuration</a>
	</body>
</html>
`

//		<script type="text/babel">
//		` + prog + `
//		</script>

//index.html
//
//	show current status
//	show button "Settings"
//
//current status:
//
//	current "on" status of each relay
//	current power in use, etc

//<html>
//<a href="/settings">Settings</a>
//<table>
//	<tr><td>Bedrooms</td><td>Relay 1</td><td>On</td></tr>
//	<tr><td></td>Relay 5</td><td>Off</td></tr>
//</table>

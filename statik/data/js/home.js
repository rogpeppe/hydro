function kWfmt(a){return(a/1e3).toFixed(3)+"kW"}function kWhfmt(a){return kWfmt(a)+"h"}function wsURL(a){var c,b=window.location;return c="https:"===b.protocol?"wss:":"ws:",c+"//"+b.host+a}var Relays=React.createClass({render:function(){return React.createElement("table",{"class":"relays"},React.createElement("thead",null,React.createElement("tr",null,React.createElement("th",null,"Cohort"),React.createElement("th",null,"Relay"),React.createElement("th",null,"Status"),React.createElement("th",null,"Since"))),React.createElement("tbody",null,this.props.relays&&this.props.relays.map(function(a){return React.createElement("tr",null,React.createElement("td",null,a.Cohort),React.createElement("td",null,React.createElement("a",{href:"/relay/"+a.Relay},a.Relay)),React.createElement("td",null,a.On?"on":"off"),React.createElement("td",null,a.Since))})))}}),Meters=React.createClass({render:function(){var a=this.props.meters;return React.createElement("div",null,React.createElement("table",{"class":"chargeable"},React.createElement("thead",null,React.createElement("tr",null,React.createElement("th",null,"Name"),React.createElement("th",null,"Chargeable power"))),React.createElement("tbody",null,React.createElement("tr",null,React.createElement("td",null,"power exported to grid"),React.createElement("td",null,kWfmt(a.Chargeable.ExportGrid))),React.createElement("tr",null,React.createElement("td",null,"export power used by Aliday"),React.createElement("td",null,kWfmt(a.Chargeable.ExportNeighbour))),React.createElement("tr",null,React.createElement("td",null,"export power used by Drynoch"),React.createElement("td",null,kWfmt(a.Chargeable.ExportHere))),React.createElement("tr",null,React.createElement("td",null,"import power used by Aliday"),React.createElement("td",null,kWfmt(a.Chargeable.ImportNeighbour))),React.createElement("tr",null,React.createElement("td",null,"import power used by Drynoch"),React.createElement("td",null,kWfmt(a.Chargeable.ImportHere))))),React.createElement("p",null),React.createElement("table",{"class":"meters"},React.createElement("thead",null,React.createElement("tr",null,React.createElement("th",null,"Meter name"),React.createElement("th",null,"Address"),React.createElement("th",null,"Current power (kW)"),React.createElement("th",null,"Total energy (kWh)"),React.createElement("th",null,"Time lag"))),React.createElement("tbody",null,a.Meters&&a.Meters.map(function(b){var c;a.Samples&&(c=a.Samples[b.Addr]);var c=a.Samples&&a.Samples[b.Addr];return React.createElement("tr",null,React.createElement("td",null,b.Name),React.createElement("td",null,React.createElement("a",{href:"/meters/"+b.Addr},b.Addr)),React.createElement("td",null,c?kWfmt(c.Power):"n/a"),React.createElement("td",null,c?kWhfmt(c.TotalEnergy):"n/a"),React.createElement("td",null,c?c.TimeLag:""))}))))}}),Reports=React.createClass({render:function(){var a=this.props.reports;return a&&0!==a.length?React.createElement("div",null,React.createElement("table",{"class":"reports"},React.createElement("thead",null,React.createElement("tr",null,React.createElement("th",null,"Name"),React.createElement("th",null,"Available reports"))),React.createElement("tbody",null," ",a.map(function(b){return React.createElement("tr",null,React.createElement("td",null,React.createElement("a",{href:b.Link,download:!0},b.Name)))})," "))):React.createElement("div",null,"No reports available")}}),socket=new ReconnectingWebSocket(wsURL("/updates",null,{timeoutInterval:5e3}));socket.onmessage=function(a){var b=JSON.parse(a.data);console.log("message",a.data);var c=document.getElementById("topLevel");console.log("toplev",c,"document",document),ReactDOM.render(React.createElement("div",null,React.createElement(Meters,{meters:b.Meters}),React.createElement("p",null),React.createElement(Relays,{relays:b.Relays}),React.createElement("p",null),React.createElement(Reports,{reports:b.Reports}),React.createElement("p",null),React.createElement("a",{href:"/config"},"Change configuration"),React.createElement("p",null),React.createElement("a",{href:"/history.html"},"Relay history")),c)};

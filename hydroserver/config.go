package hydroserver

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/errgo.v1"
)

var configTempl = newTemplate(`
<html>
<head>
<style>
.instructions {
	max-width: 30em
}
</style>
</head>
<body>
<form action="config" method="POST">
<textarea name="config" rows="30" cols="80">
{{.Store.ConfigText}}
</textarea><br>

Relay controller address <input name="relayAddr" type="text" value="{{.Controller.RelayAddr}}">
<br>
<table>
<tr><th>Meter</th><th>Addresses (space separated)</th><th>Max lag</th></tr>
<tr>
	<td>Generator</td>
	<td><input name="genMeterAddr" type="text" value="{{.GeneratorMeterAddrs | joinSp}}"></td>
	<td><input name="genMeterLag" type="text" value="{{.GeneratorAllowedLag}}"></td>
</tr>
<tr>
	<td>Aliday</td>
	<td><input name="neighbourMeterAddr" type="text" value="{{.NeighbourMeterAddrs | joinSp}}"></td>
	<td><input name="neighbourMeterLag" type="text" value="{{.NeighbourAllowedLag}}"></td>
</tr>
<tr>
	<td>Drynoch</td>
	<td><input name="hereMeterAddr" type="text" value="{{.HereMeterAddrs | joinSp}}"></td>
	<td><input name="hereMeterLag" type="text" value="{{.HereAllowedLag}}"></td>
</tr>
</table>
<br>
<input type="submit" value="Save">
<div class=instructions>
<p>
The configuration is specified as a number of lines of text.
Each line must define either a relay cohort, giving a name
to one or more relays, or a time slot, specifying a range
of times and a duration for which a cohort should be switched on.
Even though all relays in a cohort are specified together,
they may be switched on and off independently as available power
dictates.
</p>
<p>
A relay slot that defines a name for a single relay looks like this:
<br>
<tt>relay <i>number</i> is <i>name</i>.</tt>
<br>
A relay slot to define a name for several relays looks like this:
<br>
<tt>relays <i>number</i>,<i>number...</i> are <i>name</i></tt>
<br>
</p>
<p>
All text is case-insensitive and any line may contain a final full stop.
Note that relay numbers in a cohort <i>must</i> be separated
by a space.
</p>
<p>
Durations may be specified in seconds (20s), minutes (20m),
hours (2h) or a combination of the above (2h30m).
</p>
<p>
Within a time slot, a duration may specified with one of the following
qualifiers. Note that exactly when a relay is on will depend on available power.
Relays might not be switched on continuously for the specified duration.
<ul>
<li>
"for": each relay in the cohort will be switched on for exactly that amount of time in the slot.
</li>
<li>
"for at least": the cohort will be switched on for at least that amount of time in the
slot, but more if available power allows.
</li>
<li>
"for at most": the cohort
will be switched on for at most that amount of time, but perhaps
no time at all if there's no available power.
</li>
</ul>
</p>
<p>
Additional configuration parameters can be set with lines starting
with the word "config". Current parameters are:
<ul>
<li>
"cycle" (default 5m): the length of time a relay will prefer to be on before switching
to another with equal priority. This is just a preference - a relay might
switch sooner if another has higher priority.
</li>
<li>
"reaction" (default 10s): the length of time the system will wait after making
a change for the meter readings to update.
</li>
<li>
"fastest" (default 5s): the minimum length of time that the system will wait
between turning on any relay. It will never turn on more than one relay
at a time.
</li>
<p>
For example:
<p>
<tt>
relay 6 is dining room<br>
relays 0, 4, 5 are bedrooms<br>
<br>
dining room on from 14:30 to 20:45 for at least 20m<br>
bedrooms on from 17:00 to 20:00<br>
<br>
config cycle 10m
</br>
</tt>
</div>
</body>
</html>
`)

func (h *Handler) serveConfig(w http.ResponseWriter, req *http.Request) {
	log.Printf("serve %s %q", req.Method, req.URL)
	switch req.Method {
	case "GET":
		h.serveConfigGet(w, req)
	case "POST":
		h.serveConfigPost(w, req)
	default:
		badRequest(w, req, errgo.New("bad method"))
	}
}

type configTemplateParams struct {
	Store      *store
	Controller *relayCtl

	GeneratorMeterAddrs []string
	GeneratorAllowedLag time.Duration

	NeighbourMeterAddrs []string
	NeighbourAllowedLag time.Duration

	HereMeterAddrs []string
	HereAllowedLag time.Duration
}

func (h *Handler) serveConfigGet(w http.ResponseWriter, req *http.Request) {
	p := &configTemplateParams{
		Store:      h.store,
		Controller: h.controller,
	}
	for _, m := range h.store.meterState().Meters {
		switch m.Location {
		case locGenerator:
			p.GeneratorMeterAddrs = append(p.GeneratorMeterAddrs, m.Addr)
			p.GeneratorAllowedLag = m.AllowedLag
		case locNeighbour:
			p.NeighbourMeterAddrs = append(p.NeighbourMeterAddrs, m.Addr)
			p.NeighbourAllowedLag = m.AllowedLag
		case locHere:
			p.HereMeterAddrs = append(p.HereMeterAddrs, m.Addr)
			p.HereAllowedLag = m.AllowedLag
		}
	}

	var b bytes.Buffer
	if err := configTempl.Execute(&b, p); err != nil {
		log.Printf("template execution failed: %v", err)
		http.Error(w, fmt.Sprintf("template execution failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Write(b.Bytes())
}

func (h *Handler) serveConfigPost(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	configText := req.Form.Get("config")
	if err := h.store.setConfigText(configText); err != nil {
		serveConfigError(w, req, err)
		return
	}
	relayAddr := req.Form.Get("relayaddr")
	// TODO check that we can connect to the relay address?
	h.controller.SetRelayAddr(relayAddr)

	var meters []meter
	for p, info := range meterInfo {
		addrField := p + "Addr"
		lagField := p + "Lag"
		lagStr := req.Form.Get(lagField)
		allowedLag, err := time.ParseDuration(lagStr)
		if err != nil {
			badRequest(w, req, errgo.Notef(err, "invalid allowed lag duration %q (field %q; form %q)", lagStr, lagField, req.Form))
			return
		}
		addrs := strings.Fields(req.Form.Get(addrField))
		for i, addr := range addrs {
			if _, _, err := net.SplitHostPort(addr); err != nil {
				badRequest(w, req, errgo.Newf("invalid meter address %q (must be of the form host:port)", addr))
				return
			}
			name := info.name
			if len(addrs) > 1 {
				name = fmt.Sprintf("%s #%d", name, i+1)
			}
			meters = append(meters, meter{
				Name:       name,
				Location:   info.location,
				Addr:       addr,
				AllowedLag: allowedLag,
			})
		}
	}
	if err := h.store.setMeters(meters); err != nil {
		serveConfigError(w, req, err)
		return
	}

	http.Redirect(w, req, "/index.html", http.StatusMovedPermanently)
}

var meterInfo = map[string]struct {
	name     string
	location meterLocation
}{
	"genMeter": {
		name:     "Generator",
		location: locGenerator,
	},
	"hereMeter": {
		name:     "Drynoch",
		location: locHere,
	},
	"neighbourMeter": {
		name:     "Aliday",
		location: locNeighbour,
	},
}

var tmplFuncs = template.FuncMap{
	"nbsp": func(s string) string {
		return strings.Replace(s, " ", "\u00a0", -1)
	},
	"capitalize": func(s string) string {
		_, n := utf8.DecodeRuneInString(s)
		if u := strings.ToUpper(s[0:n]); u != s[0:n] {
			return u + s[n:]
		}
		return s
	},
	"joinSp": func(ss []string) string {
		return strings.Join(ss, " ")
	},
}

func newTemplate(s string) *template.Template {
	return template.Must(template.New("").Funcs(tmplFuncs).Parse(s))
}

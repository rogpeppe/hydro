package hydroserver

import (
	"bytes"
	"fmt"
	"gopkg.in/errgo.v1"
	"html/template"
	"log"
	"net/http"
)

var tmplFuncs = template.FuncMap{}

var configTempl = template.Must(template.New("").Funcs(tmplFuncs).Parse(`
<html>
<form action="config" method="POST">
	<input type="submit" value="Save">
<textarea name="config" rows="30" cols="80">
{{.ConfigText}}
</textarea>
<p>
Example configuration:
<p>
<tt>
relay 6 is dining room<br>
relays 0, 4, 5 are bedrooms<br>
<br>
dining room on from 14:30 to 20:45 for at least 20m<br>
bedrooms on from 17:00 to 20:00<br>
</tt>
</html>
`))

func (h *Handler) serveConfig(w http.ResponseWriter, req *http.Request) {
	log.Printf("serve %s %q", req.Method, req.URL)
	switch req.Method {
	case "GET":
		h.serveConfigGet(w, req)
	case "POST":
		h.serveConfigPost(w, req)
	default:
		h.badRequest(w, req, errgo.New("bad method"))
	}
}

func (h *Handler) serveConfigGet(w http.ResponseWriter, req *http.Request) {
	var b bytes.Buffer
	if err := configTempl.Execute(&b, h.store); err != nil {
		log.Printf("template execution failed: %v", err)
		http.Error(w, fmt.Sprintf("template execution failed: %v", err), http.StatusInternalServerError)
		return
	}
	w.Write(b.Bytes())
}

func (h *Handler) serveConfigPost(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	configText := req.Form.Get("config")
	if err := h.store.SetConfigText(configText); err != nil {
		// TODO serve page with errors highlit.
		h.badRequest(w, req, errgo.Newf("bad configuration: %v", err))
		return
	}
	http.Redirect(w, req, "/index.html", http.StatusMovedPermanently)
}

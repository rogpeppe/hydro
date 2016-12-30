package hydroserver

import (
	"context"
	"net/http"

	"github.com/juju/httprequest"
	"github.com/julienschmidt/httprouter"
	"github.com/rogpeppe/hydro/hydroctl"
)

var reqServer httprequest.Server

func newAPIHandler(h *Handler) http.Handler {
	r := httprouter.New()
	for _, h := range reqServer.Handlers(func(p httprequest.Params) (*apiHandler, context.Context, error) {
		return &apiHandler{h}, p.Context, nil
	}) {
		r.Handle(h.Method, h.Path, h.Handle)
	}
	return r
}

type apiHandler struct {
	h *Handler
}

type configGetRequest struct {
	httprequest.Route `httprequest:"GET /api/config"`
}

type configGetResponse struct {
	Config *hydroctl.Config
}

func (h *apiHandler) GetConfig(*configGetRequest) (*configGetResponse, error) {
	return &configGetResponse{
		Config: h.h.store.CtlConfig(),
	}, nil
}

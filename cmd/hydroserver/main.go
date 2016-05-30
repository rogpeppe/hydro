package main

import (
	"log"
	"net/http"

	"github.com/rogpeppe/hydro/eth8020test"
	"github.com/rogpeppe/hydro/hydroserver"
)

func main() {
	// TODO make not use eth8020test (actually: make the address changeable in
	// the web interface)
	srv := eth8020test.NewServer()
	h, err := hydroserver.New(hydroserver.Params{
		RelayCtlAddr: srv.Addr,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on :8089")
	err = http.ListenAndServe(":8089", h)
	log.Fatal(err)
}

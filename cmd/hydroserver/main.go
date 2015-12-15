package main

import (
	"log"
	"net/http"

	"github.com/rogpeppe/hydro/hydroserver"
)

func main() {
	h, err := hydroserver.New()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on :8089")
	err = http.ListenAndServe(":8089", h)
	log.Fatal(err)
}

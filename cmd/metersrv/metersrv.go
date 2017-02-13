package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/rogpeppe/hydro/ndmetertest"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: relaysrv [<listenaddr>]\n")
		os.Exit(2)
	}
	flag.Parse()
	addr := "localhost:0"
	if flag.NArg() > 0 {
		addr = flag.Arg(0)
	}
	srv, err := ndmetertest.NewServer(addr)
	if err != nil {
		log.Fatalf("cannot start server: %v", err)
	}
	fmt.Printf("listening on %v\n", srv.Addr)
	select {}
}

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/rogpeppe/hydro/ndmetertest"
)

var nflag = flag.Int("n", 1, "number of meter servers to start (ignored if addresses explicitly specified)")

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: metersrv [<listenaddr>...]\n")
		os.Exit(2)
	}
	flag.Parse()
	var addrs []string

	if flag.NArg() > 0 {
		addrs = flag.Args()
	} else {
		for i := 0; i < *nflag; i++ {
			addrs = append(addrs, "localhost:0")
		}
	}
	for i, addr := range addrs {
		srv, err := ndmetertest.NewServer(addr)
		if err != nil {
			log.Fatalf("cannot start server: %v", err)
		}
		fmt.Printf("%v\n", srv.Addr)
	}
	select {}
}

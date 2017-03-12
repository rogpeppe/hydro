package main

import (
	"flag"
	"fmt"
	"os"
	"log"
	"net/http"
	"path/filepath"

	"github.com/rogpeppe/hydro/eth8020test"
	"github.com/rogpeppe/hydro/hydroserver"
	"github.com/rogpeppe/hydro/ndmetertest"
)

const portBase = 44440

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: hydrotest [dir]\n")
		os.Exit(2)
	}
	flag.Parse()
	if flag.NArg() > 2 {
		flag.Usage()
	}
	dir := flag.Arg(0)
	if dir == "" {
		dir = "/tmp/hydro"
		os.MkdirAll(dir, 0777)
	}
	fmt.Printf("directory %v\n", dir)
	srv, err := eth8020test.NewServer(fmt.Sprintf("localhost:%d", portBase+1))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("relay %v\n", srv.Addr)
	for i := 0; i < 4; i++ {
		srv, err := ndmetertest.NewServer(fmt.Sprintf("localhost:%d", portBase+2+i))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("meter %d %v\n", i, srv.Addr)
	}
	h, err := hydroserver.New(hydroserver.Params{
		RelayAddrPath:   filepath.Join(dir, "relayaddr"),
		ConfigPath:      filepath.Join(dir, "relayconfig"),
		MeterConfigPath: filepath.Join(dir, "meterconfig"),
		HistoryPath:     filepath.Join(dir, "history"),
	})
	if err != nil {
		log.Fatal(err)
	}
	addr := fmt.Sprintf("localhost:%d", portBase)
	fmt.Printf("listening on http://%s\n", addr)
	err = http.ListenAndServe(addr, h)
	if err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"text/template"

	"github.com/rogpeppe/hydro/eth8020test"
	"github.com/rogpeppe/hydro/hydroserver"
	"github.com/rogpeppe/hydro/ndmetertest"
)

// bhttp put http://localhost:44442/v/ap 'v==98654'
// bhttp put http://localhost:44442/delay 'delay=25'		# in seconds

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
	}
	if _, err := os.Stat(dir); err == nil {
		fmt.Printf("existing directory %v\n", dir)
	} else {
		if err := initDir(dir); err != nil {
			log.Fatal(err)
		}
	}
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

var fileContents = map[string]string{
	"relayaddr": `{"Addr":"localhost:{{add .PortBase 1}}"}`,
	"meterconfig": `{
	"Meters": [
		{
			"Addr": "localhost:{{add .PortBase 2}}",
			"Location": 1,
			"Name": "Generator",
			"AllowedLag": 5000000000
		},
		{
			"Addr": "localhost:{{add .PortBase 3}}",
			"Location": 2,
			"Name": "Aliday"
		},
		{
			"Addr": "localhost:{{add .PortBase 4}}",
			"Location": 3,
			"Name": "Drynoch #1"
		},
		{
			"Addr": "localhost:{{add .PortBase 5}}",
			"Location": 3,
			"Name": "Drynoch #2"
		}
	]
}`,
}

type params struct {
	PortBase int
}

func initDir(dir string) error {
	fmt.Printf("initializing %v", dir)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}
	for name, cfg := range fileContents {
		tmpl, err := template.New("").Funcs(template.FuncMap{"add": add}).Parse(cfg)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, params{
			PortBase: portBase,
		}); err != nil {
			return err
		}
		if err := ioutil.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0666); err != nil {
			return err
		}
	}
	return nil
}

func add(a, b int) int {
	return a + b
}

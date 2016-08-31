package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	errgo "gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/hydroserver"
	"github.com/rogpeppe/rjson"
)

type config struct {
	RelayAddr  string
	ListenAddr string
	StateDir   string
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: hydroserver <configfile>\n")
		os.Exit(2)
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
	}
	cfg, err := readConfig(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	// TODO make the relay server address changeable in the web interface.
	h, err := hydroserver.New(hydroserver.Params{
		RelayCtlAddr: cfg.RelayAddr,
		ConfigPath:   filepath.Join(cfg.StateDir, "relayconfig"),
		HistoryPath:  filepath.Join(cfg.StateDir, "history"),
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on http://%s\n", cfg.ListenAddr)
	err = http.ListenAndServe(cfg.ListenAddr, h)
	log.Fatal(err)
}

func readConfig(f string) (*config, error) {
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, errgo.Mask(err)
	}
	var cfg config
	if rjson.Unmarshal(data, &cfg); err != nil {
		return nil, errgo.Notef(err, "cannot parse configuration file at %q", f)
	}
	if cfg.RelayAddr == "" {
		return nil, errgo.New("no relay server address set")
	}
	_, _, err = net.SplitHostPort(cfg.RelayAddr)
	if err != nil {
		return nil, errgo.Notef(err, "invalid relay address %q", cfg.RelayAddr)
	}
	if cfg.StateDir == "" {
		cfg.StateDir = "."
	}
	if _, err := os.Stat(cfg.StateDir); err != nil {
		return nil, errgo.Notef(err, "bad state directory")
	}
	if cfg.ListenAddr == "" {
		return nil, errgo.New("no listen address set")
	}
	return &cfg, nil
}

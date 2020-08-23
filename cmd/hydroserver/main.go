package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rogpeppe/rjson"
	errgo "gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/hydroserver"
)

type Config struct {
	ListenAddr string
	StateDir   string
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: hydroserver [config-file]\n")
		fmt.Fprintf(os.Stderr, "If config-file is not specified, ./hydro.cfg will be used\n")
		os.Exit(2)
	}
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
	}
	cfgFile := "hydro.cfg"
	if flag.NArg() == 1 {
		cfgFile = flag.Arg(0)
	}
	cfg, err := readConfig(cfgFile)
	if err != nil {
		log.Fatal(err)
	}
	// TODO make the time zone configurable through the UI.
	tz, err := time.LoadLocation("Europe/London")
	if err != nil {
		log.Fatal(err)
	}
	h, err := hydroserver.New(hydroserver.Params{
		RelayAddrPath:   filepath.Join(cfg.StateDir, "relayaddr"),
		ConfigPath:      filepath.Join(cfg.StateDir, "relayconfig"),
		MeterConfigPath: filepath.Join(cfg.StateDir, "meterconfig"),
		HistoryPath:     filepath.Join(cfg.StateDir, "history"),
		TZ:              tz,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on http://%s\n", cfg.ListenAddr)
	err = http.ListenAndServe(cfg.ListenAddr, h)
	log.Fatal(err)
}

func readConfig(f string) (*Config, error) {
	data, err := ioutil.ReadFile(f)
	if err != nil && !os.IsNotExist(err) {
		return nil, errgo.Mask(err)
	}
	var cfg Config
	if err == nil {
		// The config file exists so read it - otherwise we'll use all defaults.
		if rjson.Unmarshal(data, &cfg); err != nil {
			return nil, errgo.Notef(err, "cannot parse configuration file at %q", f)
		}
	}
	if cfg.StateDir == "" {
		cfg.StateDir = "."
	}
	if _, err := os.Stat(cfg.StateDir); err != nil {
		return nil, errgo.Notef(err, "bad state directory")
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	return &cfg, nil
}

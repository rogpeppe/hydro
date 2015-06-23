package main

import (
	"flag"
	"github.com/rogpeppe/hydro/eth8020"
	"log"
	"net"
	"strconv"
	"strings"
)

func main() {
	flag.Parse()
	x, err := strconv.ParseInt(strings.Replace(flag.Arg(0), " ", "", -1), 2, 32)
	if err != nil {
		log.Fatal(err)
	}

	c, err := net.Dial("tcp", "80.229.55.150:17494")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("ok")
	ec := eth8020.NewConn(c)
	//	log.Println(ec.Info())
	//	log.Println(ec.GetOutputs())
	err = ec.SetOutputs(eth8020.State(x))
	// 00000010011110001001
	if err != nil {
		log.Fatal(err)
	}

	log.Println(ec.GetOutputs())
}

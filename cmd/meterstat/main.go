package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/rogpeppe/hydro/meterstat"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: meterstat\n")
		fmt.Fprintf(os.Stderr, "Reads samples from stdin and writes them to stdout in human-readable format\n")
	}
	flag.Parse()
	if flag.NArg() != 0 {
		flag.Usage()
	}
	r := meterstat.NewSampleReader(os.Stdin)
	var prev meterstat.Sample
	for i := 0; ; i++ {
		s, err := r.ReadSample()
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "error reading sample %d: %v", i, err)
			return
		}
		if !s.Time.After(prev.Time) {
			fmt.Fprintf(os.Stderr, "warning: sample %d is out of time order (time %v is before previous %v)", i, s.Time, prev.Time)
		}
		if s.TotalEnergy < prev.TotalEnergy {
			fmt.Fprintf(os.Stderr, "warning: sample %d is out of energy order (energy %v is before previous %v)", i, s.TotalEnergy, prev.TotalEnergy)
		}
		fmt.Printf("%s %.3f\n", s.Time.Format("2006-01-02 15:04"), s.TotalEnergy/1000)
	}
}

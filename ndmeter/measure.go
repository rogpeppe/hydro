package ndmeter

import (
	"bufio"
	"math"
	"net/http"
	"regexp"
	"strconv"

	errgo "gopkg.in/errgo.v1"
)

//go:generate stringer -type measure

type measure int

const (
	_ measure = iota
	mPhase1Current
	mPhase2Current
	mPhase3Current
	mPhase1Voltage
	mPhase2Voltage
	mPhase3Voltage
	mPhase1PowerFactor
	mPhase2PowerFactor
	mPhase3PowerFactor
	mSystemkvar
	mSystemkW
	mSystemkWhHighWord
	mSystemkWhLowWord
	mSystemkvarhHighWord
	mSystemkvarhLowWord
	mPhase1kW
	mPhase2kW
	mPhase3kW
	mCurrentScale
	mVoltsScale
	mPowerScale
	mEnergyScale
	mSystemkWh
	mSystemkvarh
)

var measureLinePat = regexp.MustCompile(`<td id='([^']+)'>([^<]*)</td>`)

func Get(host string) (Reading, error) {
	resp, err := http.Get("http://" + host + "/Values_live.shtml")
	if err != nil {
		return Reading{}, errgo.Notef(err, "cannot fetch live values")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Reading{}, errgo.Newf("error status fetching live values: %v", resp.Status)
	}
	scan := bufio.NewScanner(resp.Body)
	measures := make(map[measure]int)
	for scan.Scan() {
		parts := measureLinePat.FindStringSubmatch(scan.Text())
		if len(parts) != 3 {
			continue
		}
		m, ok := measureNames[parts[1]]
		if !ok {
			continue
		}
		val, err := strconv.Atoi(parts[2])
		if err != nil {
			return Reading{}, errgo.Newf("unexpected measure value in %q", scan.Text())
		}
		measures[m] = val
	}
	systemkW, err := getVal(measures, mSystemkW, mPowerScale)
	if err != nil {
		return Reading{}, errgo.Newf("cannot read system power")
	}
	activeEnergy, err := getVal(measures, mSystemkWh, mEnergyScale)
	if err != nil {
		return Reading{}, errgo.Newf("cannot read total energy")
	}
	return Reading{
		ActivePower: systemkW * 1000,
		TotalEnergy: activeEnergy * 1000,
	}, nil
}

type Reading struct {
	// ActivePower holds the currently generated
	// power in W.
	ActivePower float64
	// TotalEnergy holds the total generated energy
	// in WH.
	TotalEnergy float64
}

func getVal(m map[measure]int, key, scale measure) (float64, error) {
	v, ok := m[key]
	if !ok {
		return 0, errgo.Newf("no key found")
	}
	sv, ok := m[scale]
	if !ok {
		return 0, errgo.Newf("no scale found")
	}
	return float64(v) * math.Pow(10, float64(sv)-6), nil
}

var measureNames = map[string]measure{
	"ap":     mSystemkW,
	"rp":     mSystemkvar,
	"v1":     mPhase1Voltage,
	"i1":     mPhase1Current,
	"pf1":    mPhase1PowerFactor,
	"p1":     mPhase1kW,
	"v2":     mPhase2Voltage,
	"i2":     mPhase2Current,
	"pf2":    mPhase2PowerFactor,
	"p2":     mPhase2kW,
	"v3":     mPhase3Voltage,
	"i3":     mPhase3Current,
	"pf3":    mPhase3PowerFactor,
	"p3":     mPhase3kW,
	"ae":     mSystemkWh,
	"re":     mSystemkvarh,
	"ascale": mCurrentScale,
	"vscale": mVoltsScale,
	"pscale": mPowerScale,
	"escale": mEnergyScale,
	// Unknown names:
	//	ct=150
	//	nv=480
	//	ps=1
}

// These registers are the ones that arrive in the fast log.
var registers = map[int] measure {
	7688: mPhase1Current,
	7689: mPhase2Current,
	7690: mPhase3Current,
	7691: mPhase1Voltage
	7692: mPhase2Voltage
	7693: mPhase3Voltage
	7702: mPhase1kW
	7703: mPhase2kW
	7704: mPhase3kW
}

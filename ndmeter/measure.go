package ndmeter

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
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

type NetworkSettings struct {
	IP             net.IP
	Subnet         net.IPMask
	DefaultGateway net.IP
	PrimaryDNS     net.IP
	SNTPServer     string
	MacAddress     net.HardwareAddr
	MeterName      string
}

func GetNetworkSettings(ctx context.Context, host string) (NetworkSettings, error) {
	r, err := getAttributes(ctx, host, "net_settings.shtml")
	if err != nil {
		return NetworkSettings{}, errgo.Notef(err, "cannot fetch live values")
	}
	defer r.close()
	var ns NetworkSettings
	for {
		attr, val, err := r.readAttr()
		if err != nil {
			break
		}
		switch attr {
		case "ip":
			ns.IP, err = parseIP(val)
		case "sn":
			ns.Subnet, err = parseIP(val)
		case "gw":
			ns.DefaultGateway, err = parseIP(val)
		case "pd":
			ns.PrimaryDNS, err = parseIP(val)
		case "ti":
			ns.SNTPServer = val
		case "ma":
			ns.MacAddress, err = net.ParseMAC(val)
		case "na":
			ns.MeterName = val
		}
		if err != nil {
			return NetworkSettings{}, fmt.Errorf("invalid value %q for attribute %q in network settings", val, attr)
		}
		// TODO TimeZone (tz)
		// TODO unknown attrs: pr, pl, pp, pe
	}
	return ns, nil
}

func parseIP(s string) ([]byte, error) {
	x, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid IP address number: %q", s)
	}
	var ip [4]byte
	binary.BigEndian.PutUint32(ip[:], uint32(x))
	return ip[:], nil
}

func Get(ctx context.Context, host string) (Reading, error) {
	r, err := getAttributes(ctx, host, "Values_live.shtml")
	if err != nil {
		return Reading{}, errgo.Notef(err, "cannot fetch live values")
	}
	defer r.close()
	measures := make(map[measure]int)
	for {
		attr, val, err := r.readAttr()
		if err != nil {
			break
		}
		m, ok := measureNames[attr]
		if !ok {
			continue
		}
		mval, err := strconv.Atoi(val)
		if err != nil {
			return Reading{}, errgo.Newf("unexpected measure value %s=%q", attr, val)
		}
		measures[m] = mval
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
	// ActivePower holds the currently used/generated
	// power in W.
	ActivePower float64
	// TotalEnergy holds the total used/generated energy
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
var registers = map[int]measure{
	7688: mPhase1Current,
	7689: mPhase2Current,
	7690: mPhase3Current,
	7691: mPhase1Voltage,
	7692: mPhase2Voltage,
	7693: mPhase3Voltage,
	7702: mPhase1kW,
	7703: mPhase2kW,
	7704: mPhase3kW,
}

var attrLinePat = regexp.MustCompile(`<td id='([^']+)'>([^<]*)</td>`)

func getAttributes(ctx context.Context, host string, page string) (*attributesReader, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+host+"/"+page, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errgo.Notef(err, "cannot fetch live values")
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, errgo.Newf("error status fetching live values: %v", resp.Status)
	}
	return &attributesReader{
		scanner: bufio.NewScanner(resp.Body),
		body:    resp.Body,
	}, nil
}

type attributesReader struct {
	scanner *bufio.Scanner
	body    io.Closer
}

func (r *attributesReader) close() {
	r.body.Close()
}

func (r *attributesReader) readAttr() (string, string, error) {
	for r.scanner.Scan() {
		parts := attrLinePat.FindStringSubmatch(r.scanner.Text())
		if len(parts) == 3 {
			return parts[1], parts[2], nil
		}
	}
	if r.scanner.Err() != nil {
		return "", "", r.scanner.Err()
	}
	return "", "", io.EOF
}

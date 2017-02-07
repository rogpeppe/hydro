package ndmeter

import "time"

type PostedData struct {
	Login Login

	// Settings is present at startup and when a change is made.
	Settings *Settings

	// Readings is present when posting is first configured and at each
	// time interval as set.
	Reading *XMLReading
}

type Login struct {
	ShortName string // A short name that identifies the meter to the remote server.
	UserName  string // A username for access to the remote server.
	Password  string // A password associated with the username.
	MAC       string // The MAC address of the meters Ethernet interface.
}

type Settings struct {
	Date       string // The current date of the meter's real time clock. (XXX format?)
	Time       string // The current time of the meter's real time clock. (XXX format?)
	Name       string // The name of the meter.
	DeviceType string // Always set to "IPM".
	Version    string // The firmware version of the metering element.
	DHCP       string // Set to 1 if the network interface is using DHCP or 0 if using static IP settings.
	IP         string // The IP address of the meter on its local network.
	Netmask    string // The network mask of the meter.
	Gateway    string // The default gateway used bhy the meter.
	DNS1       string // The meters primary DNS server.
	DNS2       string // The secondary DNS server.
	SNTP       string // The timeserver used to update the meters real time clock.
	CT         string // The meters CT primary setting.
	NV         string // The nominal voltage.
	P1         string // Pulse rate.
	MM         string // Meter model.
	MT         string // Meter type.
	FV         string // Firmware version.
	CD         string // Current demand period.
	PO         string // Pulse on time.
	HR         string // Hours run setting.
	PS         string // PT scaling factor.
	ASCALE     string // The current scaling factor.
	VSCALE     string // The voltage scaling factor.
	PSCALE     string // The power scaling factor.
	ESCALE     string // The energy scaling factor.
}

type XMLReading struct {
	Header     DateTime
	Parameters []Parameter
}

type DateTime struct {
	Date string // dd-mm-yyyy
	Time string // hh:mm
}

func (dt DateTime) Get() (time.Time, error) {
	panic("not implemented")
}

// ParamId describes a reading parameter.
type ParamId int

type Parameter struct {
	Attr  ParamId `xml:"PN"`
	Value float64 `xml:"PV"`
}

const (
	P_skWh    ParamId = 1
	P_skVAh   ParamId = 2
	P_skvarh  ParamId = 3
	P_sXkWh   ParamId = 4
	P_C1      ParamId = 5
	P_C2      ParamId = 6
	P_C3      ParamId = 7
	P_p1A     ParamId = 12
	P_p2A     ParamId = 13
	P_p3A     ParamId = 14
	P_p1V     ParamId = 15
	P_p2V     ParamId = 16
	P_p3V     ParamId = 17
	P_p12V    ParamId = 18
	P_p23V    ParamId = 19
	P_p31V    ParamId = 20
	P_sF      ParamId = 21
	P_p1PF    ParamId = 22
	P_p2PF    ParamId = 23
	P_p3PF    ParamId = 24
	P_sPF     ParamId = 25
	P_p1kW    ParamId = 26
	P_p2kW    ParamId = 27
	P_p3kW    ParamId = 28
	P_skW     ParamId = 29
	P_p1kVA   ParamId = 30
	P_p2kVA   ParamId = 31
	P_p3kVA   ParamId = 32
	P_skVA    ParamId = 33
	P_p1kvar  ParamId = 34
	P_p2kvar  ParamId = 35
	P_p3kvar  ParamId = 36
	P_skvar   ParamId = 37
	P_p1AD    ParamId = 38
	P_p2AD    ParamId = 39
	P_p3AD    ParamId = 40
	P_p1VD    ParamId = 41
	P_p2VD    ParamId = 42
	P_p3VD    ParamId = 43
	P_p1PA    ParamId = 44
	P_p2PA    ParamId = 45
	P_p3PA    ParamId = 46
	P_p1PV    ParamId = 47
	P_p2PV    ParamId = 48
	P_p3PV    ParamId = 49
	P_DkW     ParamId = 50
	P_DkVA    ParamId = 51
	P_Dkvar   ParamId = 52
	P_PHDkW   ParamId = 53
	P_PHDkVA  ParamId = 54
	P_PHDkvar ParamId = 55
	P_NI      ParamId = 56
)

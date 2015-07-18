package hydroserver

type Params struct {
	RelayAddr string
	DataDir string
}

func New(p Params) (http.Handler, error) {
	router := httprouter.New()
	
	
}

type handler struct {
	power power.Controller
	status status.Status
}


var

func (h *handler) serveHome(p httprequest.Params) {
	
}

package power

type Controller struct {
}

type State struct {
	Config []RelayConfig
	State eth8020.State
	History []StateChange
	Modes []Mode
}

type StateChange struct {
	When time.Time
	State eth8020.State
}

type RelayConfig struct {
	Id int
	Name string
	Mode string
	State bool
	History []StateChange
}

type Mode struct {
	Name string
	Kind ModeKind
}

type ModeKind string

const (
	FixedMode ModeKind = "always"
	
)

func (c *Controller) SetRelayLabel(r Relay, s string) error
func (c *Controller) SetRelayMode(r Relay, mode string) error
func (c *Controller) SetRelayStatus(r Relay, status bool) error

func (c *Controller) NewMode(name string) error
func (c *Controller) 
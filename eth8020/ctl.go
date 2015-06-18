package eth8020

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

const DefaultPort = 17494

const NumRelays = 20

type Conn struct {
	buf      []byte
	password []byte
	c        net.Conn
}

const (
	cmdModuleInfo        = 0x10
	cmdDigitalActive     = 0x20
	cmdDigitalInactive   = 0x21
	cmdDigitalSetOutputs = 0x23
	cmdDigitalGetOutputs = 0x24
	cmdDigitalGetInputs  = 0x25
	cmdGetAnalogVoltage  = 0x32
	cmdASCIITextCommand  = 0x3a
	cmdSerialNumber      = 0x77
	cmdVolts             = 0x78
	cmdLogin             = 0x79
	cmdUnlockTime        = 0x7a
	cmdLogout            = 0x7b
)

type ModuleInfo struct {
	Id        byte
	HWVersion byte
	FWVersion byte
}

var ErrFailed = errors.New("eth8030 command failed")

func NewConn(c net.Conn) *Conn {
	return &Conn{
		buf: make([]byte, 8),
		c:   c,
	}
}

func (c *Conn) Close() error {
	return c.c.Close()
}

func (c *Conn) Login(password string) error {
	c.start(cmdLogin)
	c.append([]byte(password)...)
	return c.simple()
}

func (c *Conn) Info() (ModuleInfo, error) {
	c.start(cmdModuleInfo)
	if err := c.cmd(3); err != nil {
		return ModuleInfo{}, err
	}
	return ModuleInfo{
		Id:        c.buf[0],
		HWVersion: c.buf[1],
		FWVersion: c.buf[2],
	}, nil
}

func (c *Conn) Set(relay int, on bool) error {
	if relay < 0 || relay >= NumRelays {
		return fmt.Errorf("invalid relay number %d", relay)
	}
	if on {
		c.start(cmdDigitalActive)
	} else {
		c.start(cmdDigitalInactive)
	}
	c.append(byte(relay + 1))
	c.append(0) // permanent
	return c.simple()
}

// State represents the state of all the relays in
// the device. Relay i is on if bit i is set.
// Note that relays are numbered from zero,
// unlike in the technical documentation.
type State uint

func (c *Conn) Pulse(relay int, on bool, duration time.Duration) error {
	if relay < 0 || relay >= NumRelays {
		return fmt.Errorf("invalid relay number %d", relay)
	}
	duration = duration / (100 * time.Millisecond)
	if duration <= 0 {
		duration = 1
	}
	if duration > 255 {
		return fmt.Errorf("duration out of range")
	}
	if on {
		c.start(cmdDigitalActive)
	} else {
		c.start(cmdDigitalInactive)
	}
	c.append(byte(relay + 1))
	c.append(byte(duration))
	return c.simple()
}

func (c *Conn) SetMulti(s State) error {
	c.start(cmdDigitalSetOutputs)
	c.buf = c.buf[0:4]

	c.buf[1] = byte(s >> 0)
	c.buf[2] = byte(s >> 8)
	c.buf[3] = byte(s >> 16)
	return c.simple()
}

func (c *Conn) GetOutputs() (State, error) {
	c.start(cmdDigitalGetOutputs)
	if err := c.cmd(3); err != nil {
		return 0, err
	}
	return State(c.buf[0])<<0 +
			State(c.buf[1])<<8 +
			State(c.buf[2])<<16,
		nil
}

func (c *Conn) Volts() float64 {
	panic("not implemented")
}

func (c *Conn) start(cmd byte) {
	c.buf = c.buf[:0]
	c.buf = append(c.buf, cmd)
}

func (c *Conn) append(bytes ...byte) {
	c.buf = append(c.buf, bytes...)
}

func (c *Conn) simple() error {
	if err := c.cmd(1); err != nil {
		return err
	}
	switch c.buf[0] {
	case 0:
		return nil
	case 1:
		return ErrFailed
	}
	return fmt.Errorf("unexpected response, got %d, want 0 or 1", c.buf[0])
}

func (c *Conn) cmd(nret int) error {
	if _, err := c.c.Write(c.buf); err != nil {
		return fmt.Errorf("write error: %v", err)
	}
	c.buf = c.buf[0:nret]
	_, err := io.ReadFull(c.c, c.buf)
	if err != nil {
		return fmt.Errorf("read error: %v", err)
	}
	return nil
}

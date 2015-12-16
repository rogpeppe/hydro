// Package eth8020 controls an ETH8080 relay board over a TCP
// connection. See http://www.robot-electronics.co.uk/htm/eth8020tech.htm
// for details of the device.
//
// Note that relays are consistently numbered from 0 in this
// API, not 1. This means that, for example, calling c.Set(3, true)
// will turn on the relay numbered 4 on the board.
package eth8020

import (
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// DefaultPort holds the default port that is used
// to listen for control connections on the device.
const DefaultPort = 17494

// NumRelays holds the number of relays on the device.
const NumRelays = 20

// Conn represents a control connection to the device.
type Conn struct {
	buf      []byte
	password []byte
	c        net.Conn
}

//go:generate stringer -type Cmd

type Cmd uint8

const (
	CmdModuleInfo        Cmd = 0x10
	CmdDigitalActive     Cmd = 0x20
	CmdDigitalInactive   Cmd = 0x21
	CmdDigitalSetOutputs Cmd = 0x23
	CmdDigitalGetOutputs Cmd = 0x24
	CmdDigitalGetInputs  Cmd = 0x25
	CmdGetAnalogVoltage  Cmd = 0x32
	CmdASCIITextCommand  Cmd = 0x3a
	CmdSerialNumber      Cmd = 0x77
	CmdVolts             Cmd = 0x78
	CmdLogin             Cmd = 0x79
	CmdUnlockTime        Cmd = 0x7a
	CmdLogout            Cmd = 0x7b
)

// ModuleInfo holds information about the device.
type ModuleInfo struct {
	// Id holds the id of the device.
	Id byte
	// HWVersion holds the hardware version of the device.
	HWVersion byte
	// FWVersion holds the firmware version of the device.
	FWVersion byte
}

// ErrFailed is the error returned when the device
// returns a "failed" error status.
var ErrFailed = errors.New("eth8020 command failed")

// NewConn returns a new Conn that uses the given
// connection to talk to the device. The caller
// is responsible for establishing the connection.
// The caller should not close c after calling NewConn
// (use Conn.Close instead).
func NewConn(c net.Conn) *Conn {
	return &Conn{
		buf: make([]byte, 8),
		c:   c,
	}
}

// Close closes the Conn and its underlying TCP connection.
func (c *Conn) Close() error {
	return c.c.Close()
}

// Login logs in with the given password.
func (c *Conn) Login(password string) error {
	c.start(CmdLogin)
	c.append([]byte(password)...)
	return c.simple()
}

// Info returns information on the device.
func (c *Conn) Info() (ModuleInfo, error) {
	c.start(CmdModuleInfo)
	if err := c.cmd(3); err != nil {
		return ModuleInfo{}, err
	}
	return ModuleInfo{
		Id:        c.buf[0],
		HWVersion: c.buf[1],
		FWVersion: c.buf[2],
	}, nil
}

// Set sets the given relay to the given state.
func (c *Conn) Set(relay int, on bool) error {
	if relay < 0 || relay >= NumRelays {
		return fmt.Errorf("invalid relay number %d", relay)
	}
	if on {
		c.start(CmdDigitalActive)
	} else {
		c.start(CmdDigitalInactive)
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

// Pulse changes the state of the relay for the given duration.
// Note that the duration is rounded to the nearest 100 milliseconds
// and an error is returned if the duration is longer than the
// allowed maximum (~25 seconds).
func (c *Conn) Pulse(relay int, on bool, duration time.Duration) error {
	if relay < 0 || relay >= NumRelays {
		return fmt.Errorf("invalid relay number %d", relay)
	}
	duration = (duration + 50*time.Millisecond) / (100 * time.Millisecond)
	if duration <= 0 {
		duration = 1
	}
	if duration > 255 {
		return fmt.Errorf("duration out of range")
	}
	if on {
		c.start(CmdDigitalActive)
	} else {
		c.start(CmdDigitalInactive)
	}
	c.append(byte(relay + 1))
	c.append(byte(duration))
	return c.simple()
}

// SetOutputs sets the state of all the relays.
func (c *Conn) SetOutputs(s State) error {
	c.start(CmdDigitalSetOutputs)
	c.buf = c.buf[0:4]

	c.buf[1] = byte(s >> 0)
	c.buf[2] = byte(s >> 8)
	c.buf[3] = byte(s >> 16)
	return c.simple()
}

// GetOutputs returns the state of all the relays.
func (c *Conn) GetOutputs() (State, error) {
	c.start(CmdDigitalGetOutputs)
	if err := c.cmd(3); err != nil {
		return 0, err
	}
	return State(c.buf[0])<<0 +
			State(c.buf[1])<<8 +
			State(c.buf[2])<<16,
		nil
}

// Volts returns the voltage of the low voltage power
// supply to the device.
func (c *Conn) Volts() float64 {
	panic("not implemented")
}

func (c *Conn) start(cmd Cmd) {
	c.buf = c.buf[:0]
	c.buf = append(c.buf, byte(cmd))
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

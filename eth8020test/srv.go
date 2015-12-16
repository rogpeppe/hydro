package eth8020test

import (
	"bufio"
	"io"
	"log"
	"net"
	"sync"

	"gopkg.in/errgo.v1"

	"github.com/rogpeppe/hydro/eth8020"
)

type Server struct {
	Addr string
	lis  net.Listener

	mu    sync.Mutex
	state eth8020.State
}

func NewServer() *Server {
	lis, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	srv := &Server{
		Addr: lis.Addr().String(),
		lis:  lis,
	}
	go srv.run()
	return srv
}

func (srv *Server) run() {
	for {
		c, err := srv.lis.Accept()
		if err != nil {
			return
		}
		go func() {
			err := srv.serveConn(c)
			if err != io.EOF {
				log.Printf("serveConn terminated: %v", err)
			}
		}()
	}
}

func (srv *Server) State() eth8020.State {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return srv.state
}

func (srv *Server) Close() error {
	return srv.lis.Close()
}

func (srv *Server) serveConn(conn net.Conn) error {
	r := bufio.NewReader(conn)
	for {
		cmd, err := r.ReadByte()
		if err != nil {
			return err
		}
		if err := srv.processCmd(eth8020.Cmd(cmd), r, conn); err != nil {
			return err
		}
	}
}

var (
	success = []byte{0}
	failure = []byte{1}
)

func (srv *Server) processCmd(c eth8020.Cmd, r *bufio.Reader, conn net.Conn) error {
	buf := make([]byte, 10)
	switch c {
	case eth8020.CmdDigitalSetOutputs:
		if _, err := io.ReadFull(r, buf[0:3]); err != nil {
			return errgo.Mask(err)
		}
		srv.mu.Lock()
		srv.state = eth8020.State(buf[0])<<0 +
			eth8020.State(buf[1])<<8 +
			eth8020.State(buf[2])<<16
		log.Printf("relay state set to %0*b", eth8020.NumRelays, srv.state)
		srv.mu.Unlock()
		conn.Write(success)
	case eth8020.CmdDigitalGetOutputs:
		srv.mu.Lock()
		conn.Write([]byte{
			byte(srv.state >> 0),
			byte(srv.state >> 8),
			byte(srv.state >> 16),
		})
		srv.mu.Unlock()
	default:
		log.Printf("unexpected command %v", c)
	}
	return nil
}

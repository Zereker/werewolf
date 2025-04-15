package server

import (
	"context"
	"log"
	"net"
	"sync"
	"sync/atomic"

	"github.com/Zereker/socket"
	"github.com/pkg/errors"
)

type Server struct {
	addr   *net.TCPAddr
	connID int64

	sync.RWMutex
	connections map[int64]*socket.Conn
}

func NewServer(addr string) (*Server, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, errors.New("invalid addr")
	}

	return &Server{
		addr:        tcpAddr,
		connections: make(map[int64]*socket.Conn),
	}, nil
}

func (s *Server) addConn(connID int64, conn *socket.Conn) {
	s.Lock()
	defer s.Unlock()

	log.Printf("add new conn, connID: %d, addr: %s", connID, conn.Addr())
	s.connections[connID] = conn
}

func (s *Server) deleteConn(connID int64) {
	s.Lock()
	defer s.Unlock()

	delete(s.connections, connID)
}

func (s *Server) getConn(connID int64) *socket.Conn {
	s.RLock()
	defer s.RUnlock()

	if conn, ok := s.connections[connID]; ok {
		return conn
	}

	return nil
}

func (s *Server) Decode(bytes []byte) (socket.Message, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Server) Encode(message socket.Message) ([]byte, error) {
	//TODO implement me
	panic("implement me")
}

func (s *Server) Handle(conn *net.TCPConn) {
	connID := atomic.AddInt64(&s.connID, 1)
	newConn, err := socket.NewConn(conn,
		socket.CustomCodecOption(s),
		socket.OnErrorOption(func(err error) bool {
			log.Println(err)
			return true
		}),
		socket.OnMessageOption(func(m socket.Message) error {
			conn := s.getConn(connID)
			return conn.Write(m)
		}),
	)
	if err != nil {
		panic(err)
	}

	s.addConn(connID, newConn)
	if err = newConn.Run(context.Background()); err != nil {
		s.deleteConn(connID)
	}
}

func (s *Server) Serve() error {
	server, err := socket.New(s.addr)
	if err != nil {
		return err
	}

	server.Serve(s)
	return nil
}

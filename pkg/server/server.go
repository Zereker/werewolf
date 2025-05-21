package server

import (
	"context"
	"log"
	"net"
	"sync"
	"sync/atomic"

	"encoding/json"
	"fmt"

	"github.com/Zereker/socket"
	"github.com/Zereker/werewolf/pkg/game/role" // For placeholder role
	"github.com/Zereker/werewolf/pkg/werewolf"
	"github.com/pkg/errors"
)

type Server struct {
	addr   *net.TCPAddr
	connID int64 // Atomic counter for connection IDs
	rt     *werewolf.Runtime

	sync.RWMutex
	connections   map[int64]*socket.Conn // Map connID to actual connection
	connPlayerMap map[int64]string       // Map connID to playerID
}

func NewServer(addr string, rt *werewolf.Runtime) (*Server, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, errors.New("invalid addr")
	}

	return &Server{
		addr:          tcpAddr,
		rt:            rt,
		connections:   make(map[int64]*socket.Conn),
		connPlayerMap: make(map[int64]string),
	}, nil
}

// addConn stores the connection and its associated playerID.
func (s *Server) addConn(connID int64, playerID string, conn *socket.Conn) {
	s.Lock()
	defer s.Unlock()

	log.Printf("Adding new connection: connID=%d, playerID=%s, addr=%s", connID, playerID, conn.Addr())
	s.connections[connID] = conn
	s.connPlayerMap[connID] = playerID
}

// deleteConn removes the connection and its playerID mapping.
func (s *Server) deleteConn(connID int64) {
	s.Lock()
	defer s.Unlock()

	playerID, ok := s.connPlayerMap[connID]
	if ok {
		log.Printf("Deleting connection: connID=%d, playerID=%s", connID, playerID)
		delete(s.connections, connID)
		delete(s.connPlayerMap, connID)
	} else {
		log.Printf("Attempted to delete unknown connID: %d", connID)
	}
}

// getConn retrieves a connection by its connID.
func (s *Server) getConn(connID int64) *socket.Conn {
	s.RLock()
	defer s.RUnlock()

	if conn, ok := s.connections[connID]; ok {
		return conn
	}

	return nil
}

func (s *Server) Decode(bytes []byte) (socket.Message, error) {
	var msg Message
	if err := json.Unmarshal(bytes, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	return &msg, nil
}

func (s *Server) Encode(message socket.Message) ([]byte, error) {
	msg, ok := message.(*Message)
	if !ok {
		return nil, fmt.Errorf("unexpected message type: %T", message)
	}
	bytes, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}
	return bytes, nil
}

func (s *Server) Handle(conn *net.TCPConn) {
	connID := atomic.AddInt64(&s.connID, 1) // Generate a unique ID for this connection
	playerID := fmt.Sprintf("player_%d", connID) // Create a unique player ID

	// Create a new socket connection object
	newConn, err := socket.NewConn(conn,
		socket.CustomCodecOption(s), // Use server's Encode/Decode
		socket.OnErrorOption(func(err error) bool {
			log.Printf("Connection error for connID %d, playerID %s: %v", connID, playerID, err)
			// Notify runtime about the disconnection
			s.rt.RemovePlayerConnection(playerID)
			// Remove connection from server's tracking
			s.deleteConn(connID)
			return true // Keep server running, error is handled
		}),
		socket.OnMessageOption(func(m socket.Message) error {
			serverMsg, ok := m.(*Message)
			if !ok {
				log.Printf("Received message of unexpected type from connID %d, playerID %s: %T", connID, playerID, m)
				return fmt.Errorf("unexpected message type: %T", m)
			}

			log.Printf("Received message: Type=%s, Payload=%v, connID=%d, playerID=%s", serverMsg.Type, serverMsg.Payload, connID, playerID)

			if serverMsg.Type == MessageTypePlayerAction {
				s.rt.HandlePlayerAction(playerID, serverMsg.Payload)
				// Optionally send ACK here if needed, or runtime can send updates
				return nil
			}
			
			// TODO: Handle other message types like registration if explicitly needed
			// For now, all players are auto-registered on connect.

			log.Printf("Unhandled message type from connID %d, playerID %s: %s", connID, playerID, serverMsg.Type)
			return nil // Or return an error for unhandled types
		}),
	)
	if err != nil {
		log.Printf("Failed to create new socket connection for connID %d: %v", connID, err)
		// We don't call s.addConn or s.rt.AddPlayer if NewConn fails.
		// The connection itself `conn *net.TCPConn` will be closed by the caller of Handle (socket library).
		return // Don't panic, just return. The client will fail to connect.
	}

	// Add player to runtime. Using Villager as a default role.
	// Note: player.New (and thus rt.AddPlayer) now expects a *socket.Conn.
	if err := s.rt.AddPlayer(playerID, role.NewVillager(), newConn); err != nil {
		log.Printf("Failed to add player %s (connID %d) to runtime: %v", playerID, connID, err)
		newConn.Close() // Close the connection if player couldn't be added
		// Do not call s.addConn here as the player wasn't fully set up.
		return
	}

	// If player added successfully to runtime, then add to server's tracking
	s.addConn(connID, playerID, newConn)

	// Start processing messages for this connection
	// This Run method blocks until the connection is closed or an unrecoverable error occurs.
	// The OnErrorOption above handles cleanup.
	if errRun := newConn.Run(context.Background()); errRun != nil {
		// This error is usually already logged by OnErrorOption, or it's a final error like context cancellation.
		log.Printf("Connection Run() ended for connID %d, playerID %s: %v", connID, playerID, errRun)
		// Ensure cleanup if OnErrorOption wasn't called (e.g. context cancelled)
		s.rt.RemovePlayerConnection(playerID) // Idempotent if already called
		s.deleteConn(connID)                  // Idempotent if already called
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

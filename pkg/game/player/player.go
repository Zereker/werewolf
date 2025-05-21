package player

import (
	"context"
	"errors" // Keep for Read method's placeholder error
	"fmt"    // For error wrapping in Write

	"github.com/Zereker/socket"
	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/event"
	"github.com/Zereker/werewolf/pkg/server" // For server.Message and server.MessageTypeGameUpdate
)

type player struct {
	id        string
	alive     bool
	protected bool
	role      game.Role
	conn      *socket.Conn // Connection to the client
}

// New creates a new player instance.
// The conn parameter is the client's connection.
func New(id string, role game.Role, conn *socket.Conn) game.Player {
	return &player{
		id:        id,
		alive:     true,
		protected: false,
		role:      role,
		conn:      conn,
	}
}

func (p *player) GetID() string {
	return p.id
}

func (p *player) GetRole() game.Role {
	return p.role
}

func (p *player) IsAlive() bool {
	return p.alive
}

func (p *player) SetAlive(alive bool) {
	p.alive = alive
}

func (p *player) IsProtected() bool {
	return p.protected
}

func (p *player) SetProtected(protected bool) {
	p.protected = protected
}

// Write sends an event to the player through their connection.
// It wraps the game event in a server.Message with type MessageTypeGameUpdate.
func (p *player) Write(evt event.Event[any]) error {
	if p.conn == nil {
		return errors.New("player has no active connection")
	}

	// Ensure the event's PlayerID is set, if it's meant to be specific to this player
	// or if it's a broadcast, this might be handled by the caller.
	// For now, assume evt is ready to be sent.

	serverMsg := &server.Message{
		Type:    server.MessageTypeGameUpdate,
		Payload: evt,
	}

	// The socket.Conn.Write method expects a socket.Message.
	// Our server.Message implements this.
	if err := p.conn.Write(serverMsg); err != nil {
		return fmt.Errorf("failed to write message to player %s: %w", p.id, err)
	}
	return nil
}

// Read is a placeholder for receiving client-side initiated actions for this player.
// In the current design, actions are received by the server's global OnMessageOption callback
// and then dispatched to runtime.HandlePlayerAction.
// This Read method might be used if a game phase requires specific, direct input from a player
// outside the typical action flow.
func (p *player) Read(ctx context.Context) (event.Event[any], error) {
	// For now, this is not the primary way players send actions.
	// Actions are sent as messages to the server, which forwards them to the runtime.
	return event.Event[any]{}, errors.New("player.Read not implemented, actions are handled via server messages")
}

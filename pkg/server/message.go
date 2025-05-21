package server

// MessageTypePlayerAction defines the type for player action messages.
const MessageTypePlayerAction = "PLAYER_ACTION"

// MessageTypeGameUpdate defines the type for game update messages from the runtime.
const MessageTypeGameUpdate = "GAME_UPDATE"

// MessageTypeHunterShoot defines the type for the hunter's shoot action.
const MessageTypeHunterShoot = "HUNTER_SHOOT"

// Message defines the structure for communication between client and server.
type Message struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// Type returns the message type. This is assumed to be required by the
// socket.Message interface.
func (m *Message) Type() string {
	return m.Type
}

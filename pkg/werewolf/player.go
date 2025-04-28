package werewolf

import (
	"io"
	"log/slog"

	"github.com/Zereker/werewolf/pkg/game"
)

type Player struct {
	ID string
	game.Player

	events []Event
	writer *slog.Logger
	reader io.ReadCloser
}

func New(ID string, player game.Player) *Player {
	return &Player{
		ID:     ID,
		Player: player,
		writer: slog.Default().With("user_id", ID, "player_role", player.GetRole().GetName()),
	}
}

func (p *Player) GetID() string {
	return p.ID
}

func (p *Player) Send(event Event) {
	p.events = append(p.events, event)
	p.writer.Info("player receive event", "event", event)
}

func (p *Player) Recv() Event {
	p.reader.Read()
}

package werewolf

import (
	"github.com/Zereker/werewolf/pkg/game"
)

type Player struct {
	ID string
	game.Player

	msg chan Event
}

func New(ID string, player game.Player) *Player {
	return &Player{
		ID:     ID,
		Player: player,
		msg:    make(chan Event, 1),
	}
}

func (p *Player) GetID() string {
	return p.ID
}

func (p *Player) Send(event Event) {
	p.msg <- event
}

func (p *Player) Recv() {

}

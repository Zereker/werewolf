package arean

import (
	"github.com/Zereker/werewolf/pkg/game"
)

type User struct {
	id string

	game.Player
}

func NewUser(id string, player game.Player) (User, error) {
	return User{
		id:     id,
		Player: player,
	}, nil
}

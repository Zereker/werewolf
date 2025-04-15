package arean

import (
	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/player"
)

type User struct {
	id     string
	player game.Player
}

func NewUser(id string, role game.Role, skills ...game.Skill) (User, error) {
	return User{
		id:     id,
		player: player.New(role, skills...),
	}, nil
}

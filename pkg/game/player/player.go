package player

import (
	"github.com/Zereker/werewolf/pkg/game"
)

type player struct {
	id        string
	alive     bool
	protected bool

	role game.Role
}

func New(id string, role game.Role) game.Player {
	return &player{
		id:        id,
		alive:     true,
		protected: false,
		role:      role,
	}
}

func (p *player) GetID() string {
	return p.id
}

func (p *player) GetRole() game.Role {
	return p.role
}

func (p *player) GetCamp() game.Camp {
	return p.role.GetCamp()
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

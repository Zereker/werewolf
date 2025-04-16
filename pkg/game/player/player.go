package player

import (
	"github.com/Zereker/werewolf/pkg/game"
)

type player struct {
	id        int64
	alive     bool
	protected bool

	role   game.Role
	skills []game.Skill
}

func New(id int64, role game.Role, skills ...game.Skill) game.Player {
	return &player{
		id:        id,
		alive:     true,
		protected: false,
		role:      role,
		skills:    skills,
	}
}

func (p *player) GetID() int64 {
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

func (p *player) UseSkill(phase game.PhaseType, target game.Player, skill game.Skill) error {
	return skill.Put(phase, p, target)
}

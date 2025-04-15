package player

import (
	"github.com/Zereker/werewolf/pkg/game"
)

type player struct {
	alive     bool
	protected bool

	role   game.Role
	skills []game.Skill
}

func New(role game.Role, skills ...game.Skill) game.Player {
	return &player{
		alive:     true,
		protected: false,
		role:      role,
		skills:    skills,
	}
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

func (p *player) UseSkill(phase game.Phase, target game.Player, skill game.Skill) error {
	return skill.Put(phase, p, target)
}

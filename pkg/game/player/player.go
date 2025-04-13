package player

import (
	"github.com/Zereker/werewolf/pkg/game"
)

type player struct {
	id        string
	name      string
	alive     bool
	protected bool

	role   game.Role
	skills []game.Skill
}

func New(role game.Role) game.Player {
	p := &player{
		alive:     true,
		protected: false,
		role:      role,
		skills:    make([]game.Skill, 0),
	}

	return p
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

func (p *player) AddSkill(skill game.Skill) {
	p.skills = append(p.skills, skill)
}

func (p *player) UseSkill(phase game.Phase, target game.Player, skill game.Skill) error {
	return skill.Put(phase, p, target)
}

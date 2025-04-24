package skill

import (
	"fmt"

	"github.com/Zereker/werewolf/pkg/game"
)

// Protect 守卫保护技能
type Protect struct {
	name     game.SkillType
	phase    game.PhaseType
	priority int
	hasUsed  bool
}

// NewProtectSkill creates new protect skill
func NewProtectSkill() *Protect {
	return &Protect{
		name:     game.SkillTypeProtect,
		phase:    game.PhaseNight,
		priority: PriorityProtect,
	}
}

// GetName returns skill name
func (p *Protect) GetName() game.SkillType {
	return p.name
}

// GetPhase returns skill phase
func (p *Protect) GetPhase() game.PhaseType {
	return p.phase
}

// Check 检查技能条件
func (p *Protect) Check(phase game.PhaseType, caster game.Player, target game.Player) error {
	if phase != p.phase {
		return fmt.Errorf("protect skill cannot be used in %s phase", phase)
	}

	if p.hasUsed {
		return fmt.Errorf("protect skill has already been used")
	}

	if !caster.IsAlive() {
		return fmt.Errorf("dead player cannot use protect skill")
	}

	if target == nil {
		return fmt.Errorf("protect skill requires a target")
	}

	if !target.IsAlive() {
		return fmt.Errorf("cannot protect dead target")
	}

	if target.IsProtected() {
		return fmt.Errorf("target is already protected")
	}

	return nil
}

// Put 使用保护技能
func (p *Protect) Put(caster game.Player, target game.Player) {
	p.hasUsed = true
	target.SetProtected(true)
}

// Reset resets skill state
func (p *Protect) Reset() {
	p.hasUsed = false
}

func (p *Protect) GetPriority() int {
	return p.priority
}

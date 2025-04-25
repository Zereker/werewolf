package skill

import (
	"fmt"

	"github.com/Zereker/werewolf/pkg/game"
)

// Poison 女巫毒药技能
type Poison struct {
	name     game.SkillType
	phase    game.PhaseType
	priority int
	hasUsed  bool
}

// NewPoisonSkill creates new poison skill
func NewPoisonSkill() *Poison {
	return &Poison{
		name:     game.SkillTypePoison,
		phase:    game.PhaseNight,
		priority: PriorityPoison,
	}
}

// GetName returns skill name
func (p *Poison) GetName() game.SkillType {
	return p.name
}

// GetPhase returns skill phase
func (p *Poison) GetPhase() game.PhaseType {
	return p.phase
}

// Check checks the skill conditions
func (p *Poison) Check(phase game.PhaseType, caster game.Player, target game.Player) error {
	if phase != p.phase {
		return fmt.Errorf("poison skill cannot be used in %s phase", phase)
	}

	if p.hasUsed {
		return fmt.Errorf("poison skill has already been used")
	}

	if !caster.IsAlive() {
		return fmt.Errorf("dead player cannot use poison skill")
	}

	if target == nil {
		return fmt.Errorf("poison skill requires a target")
	}

	if !target.IsAlive() {
		return fmt.Errorf("target is already dead")
	}

	if target.IsProtected() {
		return fmt.Errorf("target is protected")
	}

	return nil
}

// Put uses poison skill
func (p *Poison) Put(caster game.Player, target game.Player, option game.PutOption) {
	p.hasUsed = true
	target.SetAlive(false)
}

// Reset resets skill state
func (p *Poison) Reset() {
	p.hasUsed = false
}

func (p *Poison) GetPriority() int {
	return p.priority
}

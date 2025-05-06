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

// Check 检查技能是否可以使用
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

// Put 使用技能
func (p *Poison) Put(caster game.Player, target game.Player) {
	p.hasUsed = true
	target.SetAlive(false)
}

// Exec 执行技能，包含检查和执行两个步骤
func (p *Poison) Exec(phase game.PhaseType, caster game.Player, target game.Player) error {
	if err := p.Check(phase, caster, target); err != nil {
		return err
	}
	p.Put(caster, target)
	return nil
}

// Reset resets skill state
func (p *Poison) Reset() {
	p.hasUsed = false
}

func (p *Poison) GetPriority() int {
	return p.priority
}

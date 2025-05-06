package skill

import (
	"fmt"

	"github.com/Zereker/werewolf/pkg/game"
)

// Antidote 女巫解药技能
type Antidote struct {
	name     game.SkillType
	phase    game.PhaseType
	priority int
	hasUsed  bool
}

func NewAntidoteSkill() *Antidote {
	return &Antidote{
		name:     game.SkillTypeAntidote,
		phase:    game.PhaseNight,
		priority: PriorityAntidote,
	}
}

func (a *Antidote) GetName() game.SkillType {
	return a.name
}

func (a *Antidote) GetPhase() game.PhaseType {
	return a.phase
}

func (a *Antidote) GetPriority() int {
	return a.priority
}

// Check 检查技能是否可以使用
func (a *Antidote) Check(phase game.PhaseType, caster game.Player, target game.Player) error {
	if phase != a.phase {
		return fmt.Errorf("antidote skill cannot be used in %s phase", phase)
	}

	if a.hasUsed {
		return fmt.Errorf("antidote skill has already been used")
	}

	if !caster.IsAlive() {
		return fmt.Errorf("dead player cannot use antidote skill")
	}

	if target == nil {
		return fmt.Errorf("antidote skill requires a target")
	}

	if !target.IsAlive() {
		return fmt.Errorf("target is already dead")
	}

	return nil
}

// Put 使用技能
func (a *Antidote) Put(caster game.Player, target game.Player) {
	a.hasUsed = true
	target.SetAlive(true)
}

// Exec 执行技能，包含检查和执行两个步骤
func (a *Antidote) Exec(phase game.PhaseType, caster game.Player, target game.Player) error {
	if err := a.Check(phase, caster, target); err != nil {
		return err
	}
	a.Put(caster, target)
	return nil
}

func (a *Antidote) Reset() {
	a.hasUsed = false
}

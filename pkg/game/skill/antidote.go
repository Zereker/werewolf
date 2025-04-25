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

func (a *Antidote) Put(caster game.Player, target game.Player, option game.PutOption) {
	a.hasUsed = true
	target.SetProtected(true)
}

func (a *Antidote) Reset() {
	a.hasUsed = false
}

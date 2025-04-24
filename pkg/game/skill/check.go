package skill

import (
	"fmt"

	"github.com/Zereker/werewolf/pkg/game"
)

// Check 预言家查验技能
type Check struct {
	name     game.SkillType
	phase    game.PhaseType
	priority int
	hasUsed  bool
}

func NewCheckSkill() *Check {
	return &Check{
		name:     game.SkillTypeCheck,
		phase:    game.PhaseNight,
		priority: PriorityCheck,
	}
}

func (c *Check) GetName() game.SkillType {
	return c.name
}

func (c *Check) GetPhase() game.PhaseType {
	return c.phase
}

func (c *Check) GetPriority() int {
	return c.priority
}

func (c *Check) Check(phase game.PhaseType, caster game.Player, target game.Player) error {
	if phase != c.phase {
		return fmt.Errorf("check skill cannot be used in %s phase", phase)
	}

	if c.hasUsed {
		return fmt.Errorf("check skill has already been used")
	}

	if !caster.IsAlive() {
		return fmt.Errorf("dead seer cannot check")
	}

	if target == nil {
		return fmt.Errorf("check skill requires a target")
	}

	if !target.IsAlive() {
		return fmt.Errorf("cannot check dead target")
	}

	return nil
}

func (c *Check) Put(caster game.Player, target game.Player) {
	c.hasUsed = true
}

func (c *Check) Reset() {
	c.hasUsed = false
}

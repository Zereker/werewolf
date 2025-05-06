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

// Check 检查技能是否可以使用
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

// Put 使用技能
func (c *Check) Put(caster game.Player, target game.Player) {
	c.hasUsed = true
}

// Exec 执行技能，包含检查和执行两个步骤
func (c *Check) Exec(phase game.PhaseType, caster game.Player, target game.Player) error {
	if err := c.Check(phase, caster, target); err != nil {
		return err
	}
	c.Put(caster, target)
	return nil
}

func (c *Check) Reset() {
	c.hasUsed = false
}

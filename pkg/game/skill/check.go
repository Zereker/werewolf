package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Check represents seer's check skill
type Check struct {
	hasUsed bool // Whether skill has been used
}

// NewCheckSkill creates new check skill
func NewCheckSkill() *Check {
	return &Check{
		hasUsed: false,
	}
}

// GetName returns skill name
func (c *Check) GetName() string {
	return string(game.SkillTypeCheck)
}

// Put uses check skill
func (c *Check) Put(currentPhase game.PhaseType, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseNight {
		return errors.New("check can only be used at night")
	}

	if c.hasUsed {
		return errors.New("check has already been used")
	}

	if !target.IsAlive() {
		return errors.New("target is already dead")
	}

	// 这里应该返回查验结果，但为了保持接口一致性，我们只返回 nil
	// 实际游戏中，应该通过其他方式（如事件系统）通知预言家查验结果
	c.hasUsed = true
	return nil
}

// Reset resets skill state
func (c *Check) Reset() {
	c.hasUsed = false
}

// UseInPhase 技能使用阶段
func (c *Check) UseInPhase() game.PhaseType {
	return game.PhaseNight
}

// IsUsed 技能是否已使用
func (c *Check) IsUsed() bool {
	return c.hasUsed
}

package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Check 预言家查验技能
type Check struct {
	hasUsed bool // 是否已经使用过
}

// NewCheckSkill 创建查验技能
func NewCheckSkill() *Check {
	return &Check{
		hasUsed: false,
	}
}

// GetName 获取技能名称
func (c *Check) GetName() string {
	return string(game.SkillTypeCheck)
}

// Put 使用查验技能
func (c *Check) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseNight {
		return errors.New("只能在夜晚使用查验技能")
	}

	if c.hasUsed {
		return errors.New("今晚已经使用过查验技能")
	}

	if !target.IsAlive() {
		return errors.New("目标已经死亡")
	}

	// 这里应该返回查验结果，但为了保持接口一致性，我们只返回 nil
	// 实际游戏中，应该通过其他方式（如事件系统）通知预言家查验结果
	c.hasUsed = true
	return nil
}

// Reset 重置技能状态
func (c *Check) Reset() {
	c.hasUsed = false
}

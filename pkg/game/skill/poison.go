package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Poison 女巫毒药技能
type Poison struct {
	hasUsed bool // 是否已经使用过
}

// NewPoisonSkill 创建毒药技能
func NewPoisonSkill() *Poison {
	return &Poison{
		hasUsed: false,
	}
}

// GetName 获取技能名称
func (p *Poison) GetName() string {
	return string(game.SkillTypePoison)
}

// Put 使用毒药技能
func (p *Poison) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseNight {
		return errors.New("只能在夜晚使用毒药")
	}

	if p.hasUsed {
		return errors.New("毒药已经使用过了")
	}

	if !target.IsAlive() {
		return errors.New("目标已经死亡")
	}

	if target.IsProtected() {
		return errors.New("目标被保护，无法使用毒药")
	}

	target.SetAlive(false)
	p.hasUsed = true
	return nil
}

// Reset 重置技能状态
func (p *Poison) Reset() {
	p.hasUsed = false
}

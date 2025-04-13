package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Antidote 女巫解药技能
type Antidote struct {
	hasUsed bool // 是否已经使用过
}

// NewAntidoteSkill 创建解药技能
func NewAntidoteSkill() *Antidote {
	return &Antidote{
		hasUsed: false,
	}
}

// GetName 获取技能名称
func (a *Antidote) GetName() string {
	return string(game.SkillTypeAntidote)
}

// Put 使用解药技能
func (a *Antidote) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseNight {
		return errors.New("只能在夜晚使用解药")
	}

	if a.hasUsed {
		return errors.New("解药已经使用过了")
	}

	if !target.IsAlive() {
		return errors.New("目标已经死亡")
	}

	if target.IsProtected() {
		return errors.New("目标已经被保护")
	}

	target.SetProtected(true)
	a.hasUsed = true
	return nil
}

// Reset 重置技能状态
func (a *Antidote) Reset() {
	a.hasUsed = false
}

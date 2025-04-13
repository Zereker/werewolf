package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Kill 狼人杀人技能
type Kill struct {
	hasUsed bool // 是否已经使用过
}

// NewKillSkill 创建杀人技能
func NewKillSkill() *Kill {
	return &Kill{
		hasUsed: false,
	}
}

// GetName 获取技能名称
func (k *Kill) GetName() string {
	return string(game.SkillTypeKill)
}

// Put 使用杀人技能
func (k *Kill) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseNight {
		return errors.New("只能在夜晚使用杀人技能")
	}

	if k.hasUsed {
		return errors.New("今晚已经使用过杀人技能")
	}

	if !target.IsAlive() {
		return errors.New("目标已经死亡")
	}

	if target.IsProtected() {
		return errors.New("目标被保护，无法击杀")
	}

	target.SetAlive(false)
	k.hasUsed = true
	return nil
}

// Reset 重置技能状态
func (k *Kill) Reset() {
	k.hasUsed = false
}

package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Hunter 猎人开枪技能
type Hunter struct {
	hasUsed bool // 是否已经使用过
}

// NewHunterSkill 创建猎人开枪技能
func NewHunterSkill() *Hunter {
	return &Hunter{
		hasUsed: false,
	}
}

// GetName 获取技能名称
func (h *Hunter) GetName() string {
	return string(game.SkillTypeHunter)
}

// Put 使用猎人开枪技能
func (h *Hunter) Put(currentPhase game.PhaseType, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseNight {
		return errors.New("只能在夜晚使用猎人技能")
	}

	if h.hasUsed {
		return errors.New("今晚已经使用过猎人技能")
	}

	if !target.IsAlive() {
		return errors.New("目标已经死亡")
	}

	if target.IsProtected() {
		return errors.New("目标被保护，无法击杀")
	}

	target.SetAlive(false)
	h.hasUsed = true
	return nil
}

// Reset 重置技能状态
func (h *Hunter) Reset() {
	h.hasUsed = false
}

// UseInPhase 技能使用阶段
func (h *Hunter) UseInPhase() game.PhaseType {
	return game.PhaseNight
}

// IsUsed 技能是否已使用
func (h *Hunter) IsUsed() bool {
	return h.hasUsed
}

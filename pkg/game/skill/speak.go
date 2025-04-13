package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Speak 发言技能
type Speak struct {
	hasUsed map[game.Player]bool // 记录每个玩家是否已经发言
}

// NewSpeakSkill 创建发言技能
func NewSpeakSkill() *Speak {
	return &Speak{
		hasUsed: make(map[game.Player]bool),
	}
}

// GetName 获取技能名称
func (s *Speak) GetName() string {
	return string(game.SkillTypeSpeak)
}

// Put 使用发言技能
func (s *Speak) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseDay {
		return errors.New("只能在白天发言阶段发言")
	}

	if !caster.IsAlive() {
		return errors.New("死亡玩家不能发言")
	}

	if s.hasUsed[caster] {
		return errors.New("本轮已经发过言了")
	}

	s.hasUsed[caster] = true
	return nil
}

// Reset 重置技能状态
func (s *Speak) Reset() {
	s.hasUsed = make(map[game.Player]bool)
}

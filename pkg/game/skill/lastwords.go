package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// LastWords 遗言技能
type LastWords struct {
	hasUsed bool // 是否已经使用过
}

// NewLastWordsSkill 创建遗言技能
func NewLastWordsSkill() *LastWords {
	return &LastWords{
		hasUsed: false,
	}
}

// GetName 获取技能名称
func (l *LastWords) GetName() string {
	return string(game.SkillTypeLastWords)
}

// Put 使用遗言技能
func (l *LastWords) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseDay {
		return errors.New("只能在白天使用遗言技能")
	}

	if l.hasUsed {
		return errors.New("已经使用过遗言技能")
	}

	if caster.IsAlive() {
		return errors.New("只有死亡玩家才能使用遗言技能")
	}

	// 遗言技能不需要目标，但为了保持接口一致性，我们接受 target 参数
	l.hasUsed = true
	return nil
}

// Reset 重置技能状态
func (l *LastWords) Reset() {
	l.hasUsed = false
}

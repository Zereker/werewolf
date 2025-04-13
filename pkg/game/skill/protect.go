package skill

import (
	"errors"

	"github.com/Zereker/werewolf/pkg/game"
)

// Protect 守护技能
type Protect struct {
	hasUsed bool // 是否已经使用过
}

// NewProtectSkill 创建守护技能
func NewProtectSkill() *Protect {
	return &Protect{
		hasUsed: false,
	}
}

// GetName 获取技能名称
func (p *Protect) GetName() string {
	return string(game.SkillTypeProtect)
}

// Put 使用守护技能
func (p *Protect) Put(currentPhase game.Phase, caster game.Player, target game.Player) error {
	if currentPhase != game.PhaseNight {
		return errors.New("只能在夜晚使用守护技能")
	}

	if p.hasUsed {
		return errors.New("今晚已经使用过守护技能")
	}

	if !target.IsAlive() {
		return errors.New("目标已经死亡")
	}

	if target.IsProtected() {
		return errors.New("目标已经被保护")
	}

	target.SetProtected(true)
	p.hasUsed = true
	return nil
}

// Reset 重置技能状态
func (p *Protect) Reset() {
	p.hasUsed = false
}

package skill

import (
	"fmt"

	"github.com/Zereker/werewolf/pkg/game"
)

const (
	PriorityProtect  = 100 // 守卫技能
	PriorityAntidote = 200 // 女巫救人
	PriorityKill     = 300 // 狼人杀人
	PriorityPoison   = 400 // 女巫毒人
	PriorityCheck    = 500 // 预言家查验
	PriorityHunter   = 600 // 猎人技能（被动）

	PriorityLastWords = 100 // 遗言
	PrioritySpeak     = 200 // 普通发言

	PriorityVote = 100 // 投票
)

// New 通过技能名称创建技能对象
func New(name game.SkillType) (game.Skill, error) {
	switch name {
	case game.SkillTypeKill:
		return NewKillSkill(), nil
	case game.SkillTypeCheck:
		return NewCheckSkill(), nil
	case game.SkillTypeAntidote:
		return NewAntidoteSkill(), nil
	case game.SkillTypePoison:
		return NewPoisonSkill(), nil
	case game.SkillTypeHunter:
		return NewHunterSkill(), nil
	case game.SkillTypeSpeak:
		return NewSpeakSkill(), nil
	case game.SkillTypeVote:
		return NewVoteSkill(), nil
	case game.SkillTypeProtect:
		return NewProtectSkill(), nil
	case game.SkillTypeLastWords:
		return NewLastWordsSkill(), nil
	default:
		return nil, fmt.Errorf("unknown skill type: %s", name)
	}
}

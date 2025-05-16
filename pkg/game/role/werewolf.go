package role

import (
	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/skill"
)

// Werewolf 狼人角色
type Werewolf struct {
	skills []game.Skill
}

// NewWerewolf 创建狼人角色
func NewWerewolf() *Werewolf {
	return &Werewolf{
		skills: []game.Skill{
			skill.NewKillSkill(),      // 狼人杀人技能
			skill.NewSpeakSkill(),     // 发言技能
			skill.NewVoteSkill(),      // 投票技能
			skill.NewLastWordsSkill(), // 遗言技能
		},
	}
}

// GetName 获取角色名称
func (w *Werewolf) GetName() game.RoleType {
	return game.RoleTypeWerewolf
}

// GetCamp 获取角色所属阵营
func (w *Werewolf) GetCamp() game.Camp {
	return game.CampEvil
}

// GetAvailableSkills 获取可用技能
func (w *Werewolf) GetAvailableSkills() []game.Skill {
	return w.skills
}

// GetPriority returns role's action priority in specific phase
func (w *Werewolf) GetPriority(phase game.PhaseType) int {
	switch phase {
	case game.PhaseNight:
		return game.PriorityWerewolfNight
	case game.PhaseDay:
		return game.PriorityWerewolfDay
	case game.PhaseVote:
		return game.PriorityVote
	default:
		return game.PriorityLowest
	}
}

package role

import (
	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/skill"
)

// Hunter 猎人角色
type Hunter struct {
	skills []game.Skill
}

// NewHunter 创建猎人角色
func NewHunter() *Hunter {
	return &Hunter{
		skills: []game.Skill{
			skill.NewHunterSkill(),    // 猎人开枪技能
			skill.NewSpeakSkill(),     // 发言技能
			skill.NewVoteSkill(),      // 投票技能
			skill.NewLastWordsSkill(), // 遗言技能
		},
	}
}

// GetName 获取角色名称
func (h *Hunter) GetName() game.RoleType {
	return game.RoleTypeHunter
}

// GetCamp 获取角色所属阵营
func (h *Hunter) GetCamp() game.Camp {
	return game.CampGood
}

// GetAvailableSkills 获取可用技能
func (h *Hunter) GetAvailableSkills(phase game.PhaseType) []game.Skill {
	var result []game.Skill
	for _, s := range h.skills {
		if s.GetPhase() == phase {
			result = append(result, s)
		}
	}

	return result
}

// GetPriority returns role's action priority in specific phase
func (h *Hunter) GetPriority(phase game.PhaseType) int {
	switch phase {
	case game.PhaseNight:
		return game.PriorityHunterNight
	case game.PhaseDay:
		return game.PriorityHunterDay
	case game.PhaseVote:
		return game.PriorityVote
	default:
		return game.PriorityLowest
	}
}

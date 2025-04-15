package role

import "github.com/Zereker/werewolf/pkg/game"

// Hunter 猎人角色
type Hunter struct {
	*baseRole
}

// NewHunter 创建猎人角色
func NewHunter() *Hunter {
	return &Hunter{
		baseRole: newBaseRole(game.CampGood),
	}
}

// GetName 获取角色名称
func (r *Hunter) GetName() string {
	return string(game.RoleTypeHunter)
}

// GetAvailableSkills 获取可用技能
func (r *Hunter) GetAvailableSkills() []game.SkillType {
	return []game.SkillType{
		game.SkillTypeHunter,
		game.SkillTypeSpeak,
		game.SkillTypeVote,
		game.SkillTypeLastWords,
	}
}

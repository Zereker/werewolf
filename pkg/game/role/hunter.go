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

// GetAvailableSkills 获取可用技能
func (r *Hunter) GetAvailableSkills() []string {
	return []string{
		string(game.SkillTypeHunter),
		string(game.SkillTypeSpeak),
		string(game.SkillTypeVote),
		string(game.SkillTypeLastWords),
	}
}

// GetName 获取角色名称
func (r *Hunter) GetName() string {
	return string(game.RoleTypeHunter)
}

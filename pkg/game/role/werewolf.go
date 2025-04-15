package role

import "github.com/Zereker/werewolf/pkg/game"

// Werewolf 狼人角色
type Werewolf struct {
	*baseRole
}

// NewWerewolf 创建狼人角色
func NewWerewolf() *Werewolf {
	return &Werewolf{
		baseRole: newBaseRole(game.CampBad),
	}
}

// GetName 获取角色名称
func (r *Werewolf) GetName() string {
	return string(game.RoleTypeWerewolf)
}

// GetAvailableSkills 获取可用技能
func (r *Werewolf) GetAvailableSkills() []game.SkillType {
	return []game.SkillType{
		game.SkillTypeKill,
		game.SkillTypeSpeak,
		game.SkillTypeVote,
		game.SkillTypeLastWords,
	}
}

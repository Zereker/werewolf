package role

import "github.com/Zereker/werewolf/pkg/game"

// Seer 预言家角色
type Seer struct {
	*baseRole
}

// NewSeer 创建预言家角色
func NewSeer() *Seer {
	return &Seer{
		baseRole: newBaseRole(game.CampGood),
	}
}

// GetName 获取角色名称
func (r *Seer) GetName() string {
	return string(game.RoleTypeSeer)
}

// GetAvailableSkills 获取可用技能
func (r *Seer) GetAvailableSkills() []game.SkillType {
	return []game.SkillType{
		game.SkillTypeCheck,
		game.SkillTypeSpeak,
		game.SkillTypeVote,
		game.SkillTypeLastWords,
	}
}

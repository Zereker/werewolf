package role

import "github.com/Zereker/werewolf/pkg/game"

// Witch 女巫角色
type Witch struct {
	*baseRole
}

// NewWitch 创建女巫角色
func NewWitch() *Witch {
	return &Witch{
		baseRole: newBaseRole(game.CampGood),
	}
}

// GetAvailableSkills 获取可用技能
func (r *Witch) GetAvailableSkills() []string {
	return []string{
		string(game.SkillTypeAntidote),
		string(game.SkillTypePoison),
		string(game.SkillTypeSpeak),
		string(game.SkillTypeVote),
		string(game.SkillTypeLastWords),
	}
}

// GetName 获取角色名称
func (r *Witch) GetName() string {
	return string(game.RoleTypeWitch)
}

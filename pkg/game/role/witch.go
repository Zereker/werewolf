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

// GetName 获取角色名称
func (r *Witch) GetName() string {
	return string(game.RoleTypeWitch)
}

// GetAvailableSkills 获取可用技能
func (r *Witch) GetAvailableSkills() []game.SkillType {
	return []game.SkillType{
		game.SkillTypeAntidote,
		game.SkillTypePoison,
		game.SkillTypeSpeak,
		game.SkillTypeVote,
		game.SkillTypeLastWords,
	}
}

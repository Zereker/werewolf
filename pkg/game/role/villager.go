package role

import "github.com/Zereker/werewolf/pkg/game"

// Villager 村民角色
type Villager struct {
	*baseRole
}

// NewVillager 创建村民角色
func NewVillager() *Villager {
	return &Villager{
		baseRole: newBaseRole(game.CampGood),
	}
}

// GetName 获取角色名称
func (r *Villager) GetName() string {
	return string(game.RoleTypeVillager)
}

// GetAvailableSkills 获取可用技能
func (r *Villager) GetAvailableSkills() []game.SkillType {
	return []game.SkillType{
		game.SkillTypeSpeak,
		game.SkillTypeVote,
		game.SkillTypeLastWords,
	}
}

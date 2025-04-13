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

// GetAvailableSkills 获取可用技能
func (r *Villager) GetAvailableSkills() []string {
	return []string{
		string(game.SkillTypeSpeak),
		string(game.SkillTypeVote),
		string(game.SkillTypeLastWords),
	}
}

// GetName 获取角色名称
func (r *Villager) GetName() string {
	return string(game.RoleTypeVillager)
}

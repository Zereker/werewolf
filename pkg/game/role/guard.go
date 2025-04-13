package role

import "github.com/Zereker/werewolf/pkg/game"

// Guard 守卫角色
type Guard struct {
	*baseRole
}

// NewGuard 创建守卫角色
func NewGuard() *Guard {
	return &Guard{
		baseRole: newBaseRole(game.CampGood),
	}
}

// GetAvailableSkills 获取可用技能
func (r *Guard) GetAvailableSkills() []string {
	return []string{
		string(game.SkillTypeProtect),
		string(game.SkillTypeSpeak),
		string(game.SkillTypeVote),
		string(game.SkillTypeLastWords),
	}
}

// GetName 获取角色名称
func (r *Guard) GetName() string {
	return string(game.RoleTypeGuard)
}

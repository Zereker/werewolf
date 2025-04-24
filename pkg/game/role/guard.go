package role

import (
	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/skill"
)

// Guard 守卫角色
type Guard struct {
	skills []game.Skill
}

// NewGuard 创建守卫角色
func NewGuard() *Guard {
	return &Guard{
		skills: []game.Skill{
			skill.NewProtectSkill(),   // 守护技能
			skill.NewSpeakSkill(),     // 发言技能
			skill.NewVoteSkill(),      // 投票技能
			skill.NewLastWordsSkill(), // 遗言技能
		},
	}
}

// GetName 获取角色名称
func (g *Guard) GetName() game.RoleType {
	return game.RoleTypeGuard
}

// GetCamp 获取角色所属阵营
func (g *Guard) GetCamp() game.Camp {
	return game.CampGood
}

// GetAvailableSkills 获取可用技能
func (g *Guard) GetAvailableSkills() []game.Skill {
	return g.skills
}

package role

import (
	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/skill"
)

// Villager 村民角色
type Villager struct {
	skills []game.Skill
}

// NewVillager 创建村民角色
func NewVillager() *Villager {
	return &Villager{
		skills: []game.Skill{
			skill.NewSpeakSkill(),     // 发言技能
			skill.NewVoteSkill(),      // 投票技能
			skill.NewLastWordsSkill(), // 遗言技能
		},
	}
}

// GetName 获取角色名称
func (v *Villager) GetName() game.RoleType {
	return game.RoleTypeVillager
}

// GetCamp 获取角色所属阵营
func (v *Villager) GetCamp() game.Camp {
	return game.CampGood
}

// GetAvailableSkills 获取可用技能
func (v *Villager) GetAvailableSkills() []game.Skill {
	return v.skills
}

// GetPriority returns role's action priority in specific phase
func (v *Villager) GetPriority(phase game.PhaseType) int {
	switch phase {
	case game.PhaseNight:
		return game.PriorityVillagerNight
	case game.PhaseDay:
		return game.PriorityVillagerDay
	case game.PhaseVote:
		return game.PriorityVote
	default:
		return game.PriorityLowest
	}
}

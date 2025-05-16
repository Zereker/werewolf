package role

import (
	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/skill"
)

// Seer 预言家角色
type Seer struct {
	skills []game.Skill
}

// NewSeer 创建预言家角色
func NewSeer() *Seer {
	return &Seer{
		skills: []game.Skill{
			skill.NewCheckSkill(),     // 查验技能
			skill.NewSpeakSkill(),     // 发言技能
			skill.NewVoteSkill(),      // 投票技能
			skill.NewLastWordsSkill(), // 遗言技能
		},
	}
}

// GetName 获取角色名称
func (s *Seer) GetName() game.RoleType {
	return game.RoleTypeSeer
}

// GetCamp 获取角色所属阵营
func (s *Seer) GetCamp() game.Camp {
	return game.CampGood
}

// GetAvailableSkills 获取可用技能
func (s *Seer) GetAvailableSkills(phase game.PhaseType) []game.Skill {
	var result []game.Skill
	for _, g := range s.skills {
		if g.GetPhase() == phase {
			result = append(result, g)
		}
	}

	return result
}

// GetPriority returns role's action priority in specific phase
func (s *Seer) GetPriority(phase game.PhaseType) int {
	switch phase {
	case game.PhaseNight:
		return game.PrioritySeerNight
	case game.PhaseDay:
		return game.PrioritySeerDay
	case game.PhaseVote:
		return game.PriorityVote
	default:
		return game.PriorityLowest
	}
}

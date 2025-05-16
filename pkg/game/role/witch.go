package role

import (
	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/skill"
)

// Witch 女巫角色
type Witch struct {
	skills []game.Skill
}

// NewWitch 创建女巫角色
func NewWitch() *Witch {
	return &Witch{
		skills: []game.Skill{
			skill.NewAntidoteSkill(),  // 解药技能
			skill.NewPoisonSkill(),    // 毒药技能
			skill.NewSpeakSkill(),     // 发言技能
			skill.NewVoteSkill(),      // 投票技能
			skill.NewLastWordsSkill(), // 遗言技能
		},
	}
}

// GetName 获取角色名称
func (w *Witch) GetName() game.RoleType {
	return game.RoleTypeWitch
}

// GetCamp 获取角色所属阵营
func (w *Witch) GetCamp() game.Camp {
	return game.CampGood
}

// GetAvailableSkills 获取可用技能
func (w *Witch) GetAvailableSkills(phase game.PhaseType) []game.Skill {
	var result []game.Skill
	for _, s := range w.skills {
		if s.GetPhase() == phase {
			result = append(result, s)
		}
	}

	return result
}

// GetPriority returns role's action priority in specific phase
func (w *Witch) GetPriority(phase game.PhaseType) int {
	switch phase {
	case game.PhaseNight:
		return game.PriorityWitchNight
	case game.PhaseDay:
		return game.PriorityWitchDay
	case game.PhaseVote:
		return game.PriorityVote
	default:
		return game.PriorityLowest
	}
}

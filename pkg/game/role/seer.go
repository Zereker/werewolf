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
func (s *Seer) GetAvailableSkills() []game.Skill {
	return s.skills
}

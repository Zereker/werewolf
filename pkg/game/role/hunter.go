package role

import (
	"github.com/Zereker/werewolf/pkg/game"
	"github.com/Zereker/werewolf/pkg/game/skill"
)

// Hunter 猎人角色
type Hunter struct {
	skills []game.Skill
}

// NewHunter 创建猎人角色
func NewHunter() *Hunter {
	return &Hunter{
		skills: []game.Skill{
			skill.NewHunterSkill(),    // 猎人开枪技能
			skill.NewSpeakSkill(),     // 发言技能
			skill.NewVoteSkill(),      // 投票技能
			skill.NewLastWordsSkill(), // 遗言技能
		},
	}
}

// GetName 获取角色名称
func (h *Hunter) GetName() game.RoleType {
	return game.RoleTypeHunter
}

// GetCamp 获取角色所属阵营
func (h *Hunter) GetCamp() game.Camp {
	return game.CampGood
}

// GetAvailableSkills 获取可用技能
func (h *Hunter) GetAvailableSkills(phase game.PhaseType) []game.Skill {
	var result []game.Skill
	for _, s := range h.skills {
		// Hunter's core skill (shooting) is reactive on death, not actively used in a phase.
		// So, we exclude SkillTypeHunter from being returned for typical phases.
		// It's available for Speak, Vote, LastWords.
		if s.GetName() == game.SkillTypeHunter {
			// Only allow hunter skill if it's a special phase or context where it might be listed (e.g. info)
			// For normal night/day/vote action phases, it shouldn't be listed as an active choice.
			// However, the skill's own GetPhase() is now PhaseNone.
			// Let's assume for now that if a phase explicitly asks, it might be for info purposes.
			// The primary way it's used is reactively.
			// For now, to prevent it from being used as a regular phase action:
			if phase == game.PhaseNight || phase == game.PhaseDay {
				continue // Don't list Hunter skill as usable during these active phases
			}
		}

		// Check if the skill is appropriate for the given phase.
		// For Speak, Vote, LastWords, their GetPhase() should match game.PhaseDay, game.PhaseVote etc.
		// Hunter skill itself has game.PhaseNone, so it won't match typical phases here.
		if s.GetPhase() == phase || s.GetPhase() == game.PhaseNone { // Allow PhaseNone skills to be generally available if listed by role
			// Further refinement: PhaseNone skills might only be for specific non-standard interactions.
			// For now, if it's not Hunter skill during active phases, and phase matches, add it.
			if s.GetPhase() == phase { // Only add skills that are for the current phase.
				result = append(result, s)
			}
		}
	}
	return result
}

// GetPriority returns role's action priority in specific phase
func (h *Hunter) GetPriority(phase game.PhaseType) int {
	switch phase {
	case game.PhaseNight:
		return game.PriorityHunterNight
	case game.PhaseDay:
		return game.PriorityHunterDay
	case game.PhaseVote:
		return game.PriorityVote
	default:
		return game.PriorityLowest
	}
}

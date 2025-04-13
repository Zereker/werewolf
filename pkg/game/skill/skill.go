package skill

import (
	"fmt"

	"github.com/Zereker/werewolf/pkg/game"
)

// SkillType represents skill type
type SkillType string

const (
	// SkillTypeKill represents kill skill
	SkillTypeKill SkillType = "kill"
	// SkillTypeCheck represents check skill
	SkillTypeCheck SkillType = "check"
	// SkillTypeAntidote represents antidote skill
	SkillTypeAntidote SkillType = "antidote"
	// SkillTypePoison represents poison skill
	SkillTypePoison SkillType = "poison"
	// SkillTypeProtect represents protect skill
	SkillTypeProtect SkillType = "protect"
	// SkillTypeSpeak represents speak skill
	SkillTypeSpeak SkillType = "speak"
	// SkillTypeVote represents vote skill
	SkillTypeVote SkillType = "vote"
	// SkillTypeLastWords represents last words skill
	SkillTypeLastWords SkillType = "last_words"
	// SkillTypeHunter represents hunter skill
	SkillTypeHunter SkillType = "hunter"
)

// Skill represents game skill
type Skill interface {
	// GetName returns skill name
	GetName() string
	// Put uses skill
	Put(phase game.Phase, caster game.Player, target game.Player) error
	// Reset resets skill state
	Reset()
}

// New 通过技能名称创建技能对象
func New(name game.SkillType) (game.Skill, error) {
	switch name {
	case game.SkillTypeKill:
		return NewKillSkill(), nil
	case game.SkillTypeCheck:
		return NewCheckSkill(), nil
	case game.SkillTypeAntidote:
		return NewAntidoteSkill(), nil
	case game.SkillTypePoison:
		return NewPoisonSkill(), nil
	case game.SkillTypeHunter:
		return NewHunterSkill(), nil
	case game.SkillTypeSpeak:
		return NewSpeakSkill(), nil
	case game.SkillTypeVote:
		return NewVoteSkill(), nil
	case game.SkillTypeProtect:
		return NewProtectSkill(), nil
	case game.SkillTypeLastWords:
		return NewLastWordsSkill(), nil
	default:
		return nil, fmt.Errorf("unknown skill type: %s", name)
	}
}

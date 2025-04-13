package phase

import (
	"github.com/Zereker/werewolf/pkg/game"
)

// NightPhase represents night phase in the game
type NightPhase struct {
	*BasePhase
}

// NewNightPhase creates a new night phase
func NewNightPhase() *NightPhase {
	return &NightPhase{
		BasePhase: NewBasePhase(
			"night",
			NewDayPhase(),
			[]game.SkillType{
				game.SkillTypeKill,
				game.SkillTypeCheck,
				game.SkillTypeAntidote,
				game.SkillTypePoison,
				game.SkillTypeProtect,
			},
		),
	}
}

// ValidateAction validates if action is legal in night phase
func (p *NightPhase) ValidateAction(skill game.Skill) error {
	// Check if skill type is in available actions list
	for _, action := range p.GetAvailableActions() {
		if skill.GetName() == string(action) {
			return nil
		}
	}
	return game.ErrInvalidAction
}

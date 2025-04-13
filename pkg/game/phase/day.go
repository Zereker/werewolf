package phase

import (
	"github.com/Zereker/werewolf/pkg/game"
)

// DayPhase represents day phase in the game
type DayPhase struct {
	*BasePhase
}

// NewDayPhase creates a new day phase
func NewDayPhase() *DayPhase {
	return &DayPhase{
		BasePhase: NewBasePhase(
			"day",
			NewVotePhase(),
			[]game.SkillType{
				game.SkillTypeSpeak,
			},
		),
	}
}

// ValidateAction validates if action is legal in day phase
func (p *DayPhase) ValidateAction(skill game.Skill) error {
	// Check if skill type is in available actions list
	for _, action := range p.GetAvailableActions() {
		if skill.GetName() == string(action) {
			return nil
		}
	}
	return game.ErrInvalidAction
}
